package generate

import (
	"context"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	util "github.com/eth-easl/loader/pkg"
	fc "github.com/eth-easl/loader/pkg/function"
	mc "github.com/eth-easl/loader/pkg/metric"
	tc "github.com/eth-easl/loader/pkg/trace"
)

const (
	STATIONARY_P_VALUE     = 0.05
	FAILURE_RATE_THRESHOLD = 0.5
)

/** Seed the math/rand package for it to be different on each run. */
// func init() {
// 	rand.Seed(time.Now().UnixNano())
// }

func GenerateInterarrivalTimesInMicro(seed int, invocationsPerMinute int, uniform bool) []float64 {
	rand.Seed(int64(seed)) //! Fix randomness.
	oneSecondInMicro := 1000_000.0
	oneMinuteInMicro := 60*oneSecondInMicro - 1000

	rps := float64(invocationsPerMinute) / 60
	interArrivalTimes := []float64{}

	totoalDuration := 0.0
	for i := 0; i < invocationsPerMinute; i++ {
		var iat float64
		if uniform {
			iat = oneSecondInMicro / rps
		} else {
			iat = rand.ExpFloat64() / rps * oneSecondInMicro
		}
		//* Only guarantee microsecond-level accuracy.
		if iat < 1 {
			iat = 1
		}
		interArrivalTimes = append(interArrivalTimes, iat)
		totoalDuration += iat
	}

	if totoalDuration > oneMinuteInMicro {
		//* Normalise if it's longer than 1min.
		for i, iat := range interArrivalTimes {
			iat = iat / totoalDuration * oneMinuteInMicro
			if iat < 1 {
				iat = 1
			}
			interArrivalTimes[i] = iat
		}
	}

	// log.Info(stats.Sum(stats.LoadRawData(interArrivalTimes)))
	return interArrivalTimes
}

func GenerateStressLoads(function tc.Function, stressSlotInMinutes int, rpsStep int) {
	start := time.Now()
	wg := sync.WaitGroup{}
	exporter := mc.NewExporter()
	clusterUsage := mc.ClusterUsage{}

	/** Launch a scraper that updates the cluster usage every 15s (max. interval). */
	scrape := time.NewTicker(time.Second * 15)
	go func() {
		for {
			<-scrape.C
			clusterUsage = mc.ScrapeClusterUsage()
		}
	}()

	rps := 1
stress_generation:
	for {
		timeout := time.After(time.Minute * time.Duration(stressSlotInMinutes))
		interval := time.Duration(1000.0/rps) * time.Millisecond
		ticker := time.NewTicker(interval)
		done := make(chan bool, 1)

		/** Launch a timer. */
		go func() {
			<-timeout
			ticker.Stop()
			done <- true
		}()

		for {
			select {
			case <-ticker.C:
				go func(rps int) {
					defer wg.Done()
					wg.Add(1)

					//* Use the maximum socket timeout from AWS (1min).
					diallingTimeout := 1 * time.Minute
					ctx, cancel := context.WithTimeout(context.Background(), diallingTimeout)
					defer cancel()

					_, execRecord := fc.Invoke(ctx, function, GenerateStressExecutionSpecs)
					execRecord.Rps = rps
					execRecord.ClusterCpuAvg, execRecord.ClusterMemAvg = clusterUsage.CpuPctAvg, clusterUsage.MemoryPctAvg
					exporter.ReportExecution(execRecord)
				}(rps) //* NB: `clusterUsage` needn't be pushed onto the stack as we want the latest.

			case <-done:
				if exporter.CheckOverload(rps*60*stressSlotInMinutes, FAILURE_RATE_THRESHOLD) {
					break stress_generation
				} else {
					goto next_rps
				}
			}
		}
	next_rps:
		rps += rpsStep
		log.Info("Start next round with RPS=", rps, " after ", time.Since(start))
	}
	log.Info("Finished stress load generation with ending RPS=", rps)

	forceTimeoutDuration := 30 * time.Minute
	if wgWaitWithTimeout(&wg, forceTimeoutDuration) {
		log.Warn("Time out waiting for all invocations to return.")
	} else {
		totalDuration := time.Since(start)
		log.Info("[No time out] Total invocation + waiting duration: ", totalDuration, "\n")
	}

	defer exporter.FinishAndSave(0, 0, rps*stressSlotInMinutes)
}

func GenerateTraceLoads(
	sampleSize int,
	phaseIdx int,
	phaseOffset int,
	withBlocking bool,
	rps int,
	functions []tc.Function,
	invocationsEachMinute [][]int,
	totalNumInvocationsEachMinute []int) int {

	ShuffleAllInvocationsInplace(&invocationsEachMinute)

	isFixedRate := true
	if rps < 1 {
		isFixedRate = false
	}

	start := time.Now()
	wg := sync.WaitGroup{}
	exporter := mc.NewExporter()
	totalDurationMinutes := len(totalNumInvocationsEachMinute)

	minute := 0

trace_generation:
	for ; minute < int(totalDurationMinutes); minute++ {
		tick := 0
		var iats []float64

		rps = int(float64(totalNumInvocationsEachMinute[minute]) / 60)
		iats = GenerateInterarrivalTimesInMicro(
			minute, //! Fix randomness.
			totalNumInvocationsEachMinute[minute],
			isFixedRate,
		)
		log.Infof("Minute[%d]\t RPS=%d", minute, rps)

		numFuncInvoked := 0

		/** Set up timer to bound the one-minute invocation. */
		iterStart := time.Now()
		timeout := time.After(time.Duration(60) * time.Second)
		interval := time.Duration(iats[tick]) * time.Microsecond
		ticker := time.NewTicker(interval)
		done := make(chan bool, 2) // Two semaphores, one for timer, one for early completion.

		/** Launch a timer. */
		go func() {
			t := <-timeout
			log.Warn("(Slot finished)\t", t.Format(time.StampMilli), "\tMinute Nbr. ", minute)
			ticker.Stop()
			done <- true
		}()

		//* Bound the #invocations by `rps`.
		numInvocatonsThisMinute := util.MinOf(rps*60, totalNumInvocationsEachMinute[minute])
		var invocationCount int32

		for {
			select {
			case t := <-ticker.C:
				if tick >= numInvocatonsThisMinute {
					log.Info("Finish target invocation early at ", t.Format(time.StampMilli), "\tMinute Nbr. ", minute, " Itr. ", tick)
					done <- true
				}
				go func(m int, nxt int, phase int, rps int) {
					defer wg.Done()
					wg.Add(1)

					funcIndx := invocationsEachMinute[m][nxt]
					function := functions[funcIndx]

					//* Use the maximum socket timeout from AWS (1min).
					diallingTimeout := 1 * time.Minute
					ctx, cancel := context.WithTimeout(context.Background(), diallingTimeout)
					defer cancel()

					hasInvoked, execRecord := fc.Invoke(ctx, function, GenerateTraceExecutionSpecs)

					if hasInvoked {
						atomic.AddInt32(&invocationCount, 1)
					}
					execRecord.Phase = phase
					execRecord.Rps = rps
					exporter.ReportExecution(execRecord)
				}(minute, tick, phaseIdx, rps) //* Push vars onto the stack to prevent race.

			case <-done:
				numFuncInvoked += int(invocationCount)
				log.Info("Iteration spent: ", time.Since(iterStart), "\tMinute Nbr. ", minute)
				log.Info("Target #invocations=", totalNumInvocationsEachMinute[minute],
					" Fired #functions=", numFuncInvoked, "\tMinute Nbr. ", minute)

				invocRecord := mc.MinuteInvocationRecord{
					MinuteIdx:       minute + phaseOffset,
					Phase:           phaseIdx,
					Rps:             rps,
					Duration:        time.Since(iterStart).Microseconds(),
					NumFuncTargeted: totalNumInvocationsEachMinute[minute],
					NumFuncInvoked:  numFuncInvoked,
					NumFuncFailed:   numInvocatonsThisMinute - numFuncInvoked,
				}
				//* Export metrics for all phases.
				exporter.ReportInvocation(invocRecord)

				is_stationary := exporter.IsLatencyStationary(STATIONARY_P_VALUE)
				switch phaseIdx {
				case 3: /** Measurement phase */
					if exporter.CheckOverload(-1, FAILURE_RATE_THRESHOLD) {
						DumpOverloadFlag()
						//! Dump the flag but continue experiments.
						// minute++
						// break load_generation
					}
				default: /** Warmup phase */
					if is_stationary {
						minute++
						break trace_generation
					}
				}
				goto next_minute
			}
			//* Load the next inter-arrival time.
			tick++
			interval = time.Duration(iats[tick]) * time.Microsecond
			ticker = time.NewTicker(interval)
		}
	next_minute:
	}
	log.Info("\tFinished invoking all functions.")

	//* Hyperparameter for busy-wait
	delta := time.Duration(1) //TODO: Make this force wait customisable (currently it's the same duration as that of the traces).
	forceTimeoutDuration := time.Duration(totalDurationMinutes) * time.Minute / delta

	if !withBlocking {
		forceTimeoutDuration = time.Second * 1
	}

	if wgWaitWithTimeout(&wg, forceTimeoutDuration) {
		log.Warn("Time out waiting for all invocations to return.")
	} else {
		totalDuration := time.Since(start)
		log.Info("[No time out] Total invocation + waiting duration: ", totalDuration, "\n")
	}

	defer exporter.FinishAndSave(sampleSize, phaseIdx, minute)
	return phaseOffset + minute
}

/**
 * This function waits for the waitgroup for the specified max timeout.
 * Returns true if waiting timed out.
 */
func wgWaitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	log.Info("Start waiting for all requests to return.")
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		log.Info("Finished waiting for all invocations.")
		return false
	case <-time.After(timeout):
		return true
	}
}

/**
 * This function has/uses side-effects, but for the sake of performance
 * keep it for now.
 */
func ShuffleAllInvocationsInplace(invocationsEachMinute *[][]int) {
	suffleOneMinute := func(invocations *[]int) {
		for i := range *invocations {
			rand.Seed(int64(i)) //! Fix randomness.

			j := rand.Intn(i + 1)
			(*invocations)[i], (*invocations)[j] = (*invocations)[j], (*invocations)[i]
		}
	}

	for minute := range *invocationsEachMinute {
		suffleOneMinute(&(*invocationsEachMinute)[minute])
	}
}

func GenerateStressExecutionSpecs(function tc.Function) (int, int) {
	//* Median values of corresponding avg. of the whole Azure trace.
	return 4443, 1420
}

func GenerateTraceExecutionSpecs(function tc.Function) (int, int) {
	seed := util.Hex2Int(function.Hash)
	rand.Seed(seed) //! Fix randomness.

	var runtime, memory int
	//* Generate a uniform quantiles in [0, 1).
	memQtl := rand.Float64()
	runQtl := rand.Float64()
	runStats := function.RuntimeStats
	memStats := function.MemoryStats

	/**
	 * With 50% prob., returns average values (since we sample the trace based upon the average)
	 * With 50% prob., returns uniform volumns from the the upper and lower percentile interval.
	 */
	if memory = memStats.Average; util.RandBool() {
		switch {
		case memQtl <= 0.01:
			memory = memStats.Percentile1
		case memQtl <= 0.05:
			memory = util.RandIntBetween(memStats.Percentile1, memStats.Percentile5)
		case memQtl <= 0.25:
			memory = util.RandIntBetween(memStats.Percentile5, memStats.Percentile25)
		case memQtl <= 0.50:
			memory = util.RandIntBetween(memStats.Percentile25, memStats.Percentile50)
		case memQtl <= 0.75:
			memory = util.RandIntBetween(memStats.Percentile50, memStats.Percentile75)
		case memQtl <= 0.95:
			memory = util.RandIntBetween(memStats.Percentile75, memStats.Percentile95)
		case memQtl <= 0.99:
			memory = util.RandIntBetween(memStats.Percentile95, memStats.Percentile99)
		case memQtl < 1:
			memory = util.RandIntBetween(memStats.Percentile99, memStats.Percentile100)
		}
	}

	if runtime = runStats.Average; util.RandBool() {
		switch {
		case runQtl <= 0.01:
			runtime = runStats.Percentile0
		case runQtl <= 0.25:
			runtime = util.RandIntBetween(runStats.Percentile1, runStats.Percentile25)
		case runQtl <= 0.50:
			runtime = util.RandIntBetween(runStats.Percentile25, runStats.Percentile50)
		case runQtl <= 0.75:
			runtime = util.RandIntBetween(runStats.Percentile50, runStats.Percentile75)
		case runQtl <= 0.95:
			runtime = util.RandIntBetween(runStats.Percentile75, runStats.Percentile99)
		case runQtl <= 0.99:
			runtime = util.RandIntBetween(runStats.Percentile99, runStats.Percentile100)
		case runQtl < 1:
			// 100%ile is smaller from the max. somehow.
			runtime = util.RandIntBetween(runStats.Percentile100, runStats.Maximum)
		}
	}
	return runtime, memory
}

func DumpOverloadFlag() {
	// If the file doesn't exist, create it, or append to the file
	_, err := os.OpenFile("overload.flag", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
}
