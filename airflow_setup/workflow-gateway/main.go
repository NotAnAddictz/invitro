package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/apache/airflow-client-go/airflow"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func getFinishedDagRun(request airflow.ApiGetDagRunRequest, c chan string) {
	// poll airflow until DagRun is done
	for {
		dagrun, _, err := request.Execute()
		if err != nil {
			fmt.Println(err)
		} else {
			if *dagrun.State == airflow.DAGSTATE_SUCCESS || *dagrun.State == airflow.DAGSTATE_FAILED {
				c <- *dagrun.DagRunId.Get()
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func startDagRun(apiBaseUrl string, dagId string) (airflow.DAGRun, error) {
	return startDagRunWithConf(apiBaseUrl, dagId, []byte(`{}`))
}

func startDagRunWithConf(apiBaseUrl string, dagId string, conf []byte) (airflow.DAGRun, error) {
	endpointUrl := apiBaseUrl + "/dags/" + dagId + "/dagRuns"
	req, err := http.NewRequest("POST", endpointUrl, bytes.NewBuffer(conf))
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth("admin", "admin")
	client := &http.Client{}
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	dagrun := airflow.DAGRun{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(body, &dagrun)
	if err != nil {
		panic(err)
	}
	return dagrun, err
}

func runWorkflow(ginCtx *gin.Context, conf *airflow.Configuration, cli *airflow.APIClient) {
	dagId := ginCtx.Param("dagId")
	dagInput := make(map[string]any)
	if err := ginCtx.BindJSON(&dagInput); err != nil {
		fmt.Println(err)
		return
	}
	dagConf := make(map[string]map[string]any)
	dagConf["conf"] = make(map[string]any)
	dagConf["conf"]["data"] = dagInput["input"]
	serializedDagConf, err := json.Marshal(dagConf)
	if err != nil {
		fmt.Println(err)
		return
	}

	// forward basic auth to airflow webserver
	authHeader := ginCtx.GetHeader("Authorization")
	basicAuthFields := strings.Fields(authHeader) // split into ["Basic", <b64 encoded username and password>]
	if len(basicAuthFields) != 2 {
		ginCtx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	decodedBasicAuth, err := base64.StdEncoding.DecodeString(basicAuthFields[1])
	if err != nil {
		ginCtx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	basicAuth := strings.Split(string(decodedBasicAuth), ":") // split into username and password
	if len(basicAuth) != 2 {
		ginCtx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	cred := airflow.BasicAuth{
		UserName: basicAuth[0],
		Password: basicAuth[1],
	}
	ctx := context.WithValue(context.Background(), airflow.ContextBasicAuth, cred)

	// Unpause all dags by PATCHing them with an unpaused DAG.

	// This unfortunately seems to be the most elegant way to get a NullableBool that is set to false.
	tmp := airflow.NullableBool{}
	tmp2 := false
	tmp.Set(&tmp2)
	// Create unpaused DAG and PATCH all dags (dagId '~' matches all dags). UpdateMask ensures that only the
	// 'is_paused' field is updated.
	unpausedDag := airflow.DAG{IsPaused: tmp}
	req := cli.DAGApi.PatchDags(ctx).DagIdPattern("~").DAG(unpausedDag).UpdateMask([]string{"is_paused"})
	_, _, err = req.Execute()
	if err != nil {
		fmt.Println("Failed to unpause DAGs")
		fmt.Println(err)
		ginCtx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Ideally, we could use the Airflow API client as shown below to start a DAGRun, however, this part of the API
	// currently seems to be broken.
	// dagrun, _, err := cli.DAGRunApi.PostDagRun(ctx, "etl_example").Execute()
	u := url.URL{Host: conf.Host, Scheme: "http", Path: "/api/v1"}
	dagrun, err := startDagRunWithConf(u.String(), dagId, serializedDagConf)
	if err != nil {
		fmt.Println(err)
		ginCtx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c := make(chan string)

	go getFinishedDagRun(cli.DAGRunApi.GetDagRun(ctx, *dagrun.DagId, *dagrun.DagRunId.Get()), c)
	<-c

	// find leaf task from which we retrieve the output
	taskCollection, _, err := cli.DAGApi.GetTasks(ctx, dagId).Execute()
	if err != nil {
		fmt.Println(err)
		ginCtx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tasks := taskCollection.GetTasks()
	leafTaskId := ""
	for i := 0; i < len(tasks); i++ {
		task := tasks[i]
		downstreamTaskIds := *(task.DownstreamTaskIds)
		if len(downstreamTaskIds) == 0 {
			fmt.Printf("Found leaf task '%s'\n", *task.TaskId)
			leafTaskId = *task.TaskId
			break
		}
	}

	// load output from leaf task
	returnValue, _, err := cli.XComApi.GetXcomEntry(ctx, *dagrun.DagId, *dagrun.DagRunId.Get(), leafTaskId, "return_value").Execute()
	if err != nil {
		fmt.Println(err)
		ginCtx.AbortWithStatus(http.StatusInternalServerError)
		return
	} else {
		fmt.Printf("Workflow output: %s\n", *returnValue.Value)
	}
	outputData := make(map[string]any)
	var retVal any
	err = json.Unmarshal([]byte(*returnValue.Value), &retVal)
	if err != nil {
		fmt.Println(err)
		ginCtx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	outputData["output"] = retVal
	ginCtx.JSON(http.StatusOK, outputData)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: workflow_gateway '<gateway ip>:<gateway port>' '<airflow webserver host>:<airflow webserver port>'")
		os.Exit(1)
	}
	conf := airflow.NewConfiguration()
	conf.Host = os.Args[2]
	conf.Scheme = "http"
	cli := airflow.NewAPIClient(conf)

	router := gin.Default()
	router.POST("/runWorkflow/:dagId", func(c *gin.Context) {
		start := time.Now()
		runWorkflow(c, conf, cli)
		duration := time.Since(start)
		fmt.Printf("Running %s took %s\n", c.Param("dagId"), duration)
	})

	router.Run(os.Args[1])
}
