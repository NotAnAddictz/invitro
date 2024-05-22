package generator
import (
	"github.com/vhive-serverless/loader/pkg/common"
	"container/list"
	"strconv"
	"strings"
	"os"
	"math/rand"
	"encoding/csv"
	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/config"
	"fmt"
	
)
// Visual Representation for the DAG
func printDAG(DAGWorkflow *list.List) {
	DAGNode := DAGWorkflow.Front()
	nodeQueue := make([]*list.Element, 0)
	nodeQueue = append(nodeQueue, DAGNode)
	var printMessage string
	var buffer string = ""
	var dummyNode *list.Element
	var startingNode bool = true
	for len(nodeQueue) > 0 {
		DAGNode = nodeQueue[0]
		nodeQueue = nodeQueue[1:]
		functionId := common.GetName(DAGNode.Value.(*common.Node).Function)
		if startingNode {
			printMessage = "|" + strconv.Itoa(functionId)
			for i := 0; i < DAGNode.Value.(*common.Node).Depth; i++ {
				buffer += "     "
			}
			printMessage = buffer + printMessage
			startingNode = false
		} else {
			printMessage = printMessage + " -> " + strconv.Itoa(functionId)
		}
		for i := 0; i < len(DAGNode.Value.(*common.Node).Branches); i++ {
			nodeQueue = append(nodeQueue, dummyNode)
			copy(nodeQueue[1:], nodeQueue)
			nodeQueue[0] = DAGNode.Value.(*common.Node).Branches[i].Front()
		}
		if DAGNode.Next() == nil {
			println(printMessage)
			buffer = ""
			if len(nodeQueue) > 0 {
				startingNode = true
			} else {
				break
			}
		} else {
			nodeQueue = append(nodeQueue, dummyNode)
			copy(nodeQueue[1:], nodeQueue)
			nodeQueue[0] = DAGNode.Next()
		}
	}
}

// Read the Cumulative Distribution Frequency (CDF) of the widths and depths of a DAG
func generateCDF(file string) [][]float64 {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	records = records[1:]
	cdf := make([][]float64, len(records[0]))
	for i := 0; i < len(records[0]); i++ {
		cdf[i] = make([]float64, len(records))
	}
	for i := 0; i < len(records[0]); i += 2 {
		for j := 0; j < len(records); j++ {
			cdfProb, _ := strconv.ParseFloat(strings.TrimSuffix(records[j][i+1], "%"), 64)
			cdfValue, _ := strconv.ParseFloat(records[j][i], 64)
			cdf[i+1][j] = cdfProb
			cdf[i][j] = cdfValue
			if cdfProb == 100.00 {
				cdf[i] = cdf[i][:j+1]
				cdf[i+1] = cdf[i+1][:j+1]
				break
			}
		}
	}
	return cdf
}

// Generate pseudo-random probabilities and compare it with the given CDF to obtain the depth and width of the DAG
func getDAGStats(cdf [][]float64, maxSize int, numberOfTries int) (int, int) {
	var width, depth int
	depthProb := rand.Float64() * 100
	widthProb := rand.Float64() * 100
	for i, value := range cdf[1] {
		if value >= widthProb {
			width = int(cdf[0][i])
			break
		}
	}
	for i, value := range cdf[3] {
		if value >= depthProb {
			depth = int(cdf[2][i])
			break
		}
	}
	// Re-run DAG Generation if size exceeds number of functions
	if maxSize < width*(depth-1)+1 {
		if numberOfTries == 10 {
			return 1, maxSize
		}
		width, depth = getDAGStats(cdf, maxSize, numberOfTries+1)
	}
	return width, depth
}

func GetMaxInvocation(functionList []*common.Function) []int {
	maxInvocation := make([]int, len(functionList[0].InvocationStats.Invocations))
	for _, i := range functionList {
		for index, invocation := range i.InvocationStats.Invocations {
			maxInvocation[index] = max(maxInvocation[index], invocation)
		}
	}
	return maxInvocation
}

func GenerateDAG(config *config.LoaderConfiguration,functions []*common.Function) []*list.List{
	var width,depth int
	DAGDistribution := generateCDF(fmt.Sprintf("%s/dag_structure.csv", config.TracePath))
	totalLinkedList := []*list.List{}
	for _, function := range functions {
		if config.EnableDAGDataset{
			width, depth = getDAGStats(DAGDistribution, len(functions), 0)
		} else {
			// Sanity checking if max size of DAG exceeds number of functions available
			width = config.Width
			depth = config.Depth
			if len(functions) < (depth-1)*width+1 {
				log.Fatalf("DAG size exceeded: Functions required: %d, Available Functions: %d", (depth-1)*width+1, len(functions))
			}
		}
		functionLinkedList := createDAGWorkflow(functions, function, width, depth)
		printDAG(functionLinkedList)
		totalLinkedList = append(totalLinkedList, functionLinkedList)
	}
	return totalLinkedList
}

func createDAGWorkflow(functionList []*common.Function, function *common.Function, maxWidth int, maxDepth int) *list.List {
	DAGList := list.New()
	if maxDepth == 1 {
		DAGList.PushBack(&common.Node{Function: function, Depth: 0})
		return DAGList
	}
	widthList := generateNodeDistribution(maxWidth, maxDepth)
	// Implement a FIFO queue for nodes to assign functions and branches to each node.
	nodeQueue := []*list.Element{}
	for i := 0; i < len(widthList); i++ {
		widthList[i] -= 1
		DAGList.PushBack(&common.Node{Depth: -1})
	}
	var functionID int = common.GetName(function)
	DAGList.Front().Value = &common.Node{Function: function, Depth: 0}
	functionID = (functionID + 1) % len(functionList)
	nodeQueue = append(nodeQueue, DAGList.Front())
	for len(nodeQueue) > 0 {
		listElement := nodeQueue[0]
		nodeQueue = nodeQueue[1:]
		node := listElement.Value.(*common.Node)
		// Checks if the node has reached the maximum depth of the DAG (maxDepth -1)
		if node.Depth == maxDepth-1 {
			continue
		}
		child := &common.Node{Function: functionList[functionID], Depth: node.Depth + 1}
		functionID = (functionID + 1) % len(functionList)
		listElement.Next().Value = child
		nodeQueue = append(nodeQueue, listElement.Next())
		// Creating parallel branches from the node, if width of next stage > width of current stage
		var nodeList []*list.List
		if widthList[node.Depth+1] > 0 {
			nodeList, nodeQueue = addBranches(nodeQueue, widthList, node, functionList, functionID)
			functionID = (functionID + len(nodeList)) % len(functionList)
		} else {
			nodeList = []*list.List{}
		}
		node.Branches = nodeList
	}
	return DAGList
}

func addBranches(nodeQueue []*list.Element, widthList []int, node *common.Node, functionList []*common.Function, functionID int) ([]*list.List, []*list.Element) {
	var additionalBranches int
	if len(nodeQueue) < 1 || (nodeQueue[0].Value.(*common.Node).Depth > node.Depth) {
		additionalBranches = widthList[node.Depth+1]
	} else {
		additionalBranches = rand.Intn(widthList[node.Depth+1] + 1)
	}
	for i := node.Depth + 1; i < len(widthList); i++ {
		widthList[i] -= additionalBranches
	}
	nodeList := make([]*list.List, additionalBranches)
	for i := 0; i < additionalBranches; i++ {
		newBranch := createNewBranch(functionList, node, len(widthList), functionID)
		functionID = (functionID + 1) % len(functionList)
		nodeList[i] = newBranch
		nodeQueue = append(nodeQueue, newBranch.Front())
	}
	return nodeList, nodeQueue
}

func createNewBranch(functionList []*common.Function, node *common.Node, maxDepth int, functionID int) *list.List {
	DAGBranch := list.New()
	// Ensuring that each node is connected to a child until the maximum depth
	for i := node.Depth + 1; i < maxDepth; i++ {
		DAGBranch.PushBack(&common.Node{Depth: -1})
	}
	child := &common.Node{Function: functionList[functionID], Depth: node.Depth + 1}
	DAGBranch.Front().Value = child
	return DAGBranch
}

func generateNodeDistribution(maxWidth int, maxDepth int) []int {
	// Generating the number of nodes per depth (stage).
	widthList := []int{}
	widthList = append(widthList, 1)
	for i := 1; i < maxDepth-1; i++ {
		widthList = append(widthList, (rand.Intn(maxWidth-widthList[i-1]+1) + widthList[i-1]))
	}
	widthList = append(widthList, maxWidth)
	return widthList
}