package main

import (
	"fcmeans/common"
	"path/filepath"
	"encoding/csv"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"reflect"
	"errors"
	"github.com/ergo-services/ergo"
	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/ergo-services/ergo/node"
	"encoding/json"

)
type FCMeansClient struct {
	*common.FedLangProcess
	factor_lambda float64
	num_clients   int
	target_feature string
	X             [][]float64
	y_true        []float64
	rows          int
	num_features  int
	datasetName   string

}
type FLExperiment struct {
	_global_model_parameters    [][]float64
	_client_configuration       map[string]interface{}
	_max_number_rounds          int
	_client_ids                 interface{}
	_type_of_termination        string
	_termination_threshold      float64
	_monitored_metric           interface{}
	_type_of_client_selection   interface{}
	_client_ratio               interface{}
	_client_values              interface{}
	_step_wise_client_selection bool
	_client_selection_threshold interface{}
	_round_selected_ids         []int
	_latency                    int
	_calls_list                 []etf.Tuple
	_validate                   bool
	experiment_history          interface{}
	_latency_required           bool
}
type ExperimentConfig struct {
	LambdaFactor  float64 `json:"lambdaFactor"`
	NumClients    int     `json:"numClients"`
	TargetFeature string  `json:"targetFeature"`
	DatasetName   string  `json:"datasetName"`
}
func load_experiment_info(numClients int, targetFeature string, datasetName ...string) ([][]float64, []float64, string) {
	basePath := filepath.Join(os.Getenv("PROJECT_PATH"), "datasets")
	var datasetPath string
	if len(datasetName) == 0 {
		datasetPath = filepath.Join(basePath, "pendigits.csv")
	} else {
		datasetPath = filepath.Join(basePath, datasetName[0])
	}

	file, err := os.Open(datasetPath)
	if err != nil {
		log.Fatalf("Failed to open dataset file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read dataset file: %v", err)
	}

	size := int(math.Ceil(float64(len(records)-1) / float64(numClients)))
	log.Printf("dataset chunk size = %d", size)

	header := records[0]
	targetIndex := -1
	for i, col := range header {
		if col == targetFeature {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		log.Fatalf("Target feature %s not found in dataset", targetFeature)
	}

	var dfX [][]float64
	var yTrue []float64
	for _, record := range records[1:] {
		var row []float64
		for i, value := range record {
			val, err := strconv.ParseFloat(value, 64)
			if err != nil {
				log.Fatalf("Failed to parse value: %v", err)
			}
			if i == targetIndex {
				yTrue = append(yTrue, val)
			} else {
				row = append(row, val)
			}
		}
		dfX = append(dfX, row)
	}

	randomSamples := rand.Perm(len(dfX))[:size]
	var values [][]float64
	var yTrueSample []float64
	for _, idx := range randomSamples {
		values = append(values, dfX[idx])
		yTrueSample = append(yTrueSample, yTrue[idx])
	}

	log.Printf("dataset chunk size = %d, X.shape = (%d, %d), y.shape = (%d)", size, len(dfX), len(dfX[0]), len(yTrueSample))
	return values, yTrueSample, targetFeature
}
func (s *FCMeansClient) Call(funcName string, params ...interface{}) (result interface{}, err error) {
	StubStorage := map[string]interface{}{
		"init_client":    s.init_client,
		"process_client": s.process_client,
		"destroy":         s.destroy,
	}

	log.Printf("funcname = %s\n", funcName) // = %v\n", funcName, params)

	f := reflect.ValueOf(StubStorage[funcName])
	if len(params) != f.Type().NumIn() {
		err = errors.New("The number of params is out of index.")
		return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		// log.Printf("param[%d] = %#v\n", k, param)
		in[k] = reflect.ValueOf(param)
	}
	var res []reflect.Value
	log.Printf("Calling %#v with in_args %#v\n", f, in)
	res = f.Call(in)
	if len(res) == 0 {
		err = nil
		return
	}
	result = res[0].Interface()
	return
}
func (f *FCMeansClient) init_client(experiment,json_str_config string) {
	var experimentConfig ExperimentConfig
	err := json.Unmarshal([]byte(json_str_config), &experimentConfig)
	if err != nil {
		log.Fatalf("Error parsing JSON config: %v", err)
	}

	log.Printf("experiment = %s, experimentConfig = %+v", experiment, experimentConfig)

	f.factor_lambda = experimentConfig.LambdaFactor
	f.num_clients = experimentConfig.NumClients
	f.target_feature = experimentConfig.TargetFeature
	f.datasetName = experimentConfig.DatasetName

	f.X, f.y_true, f.target_feature = load_experiment_info(f.num_clients, f.target_feature, f.datasetName)
	f.rows, f.num_features = len(f.X), len(f.X[0])
	f.Process.Send(
		gen.ProcessID{Name: f.Erl_worker_mailbox, Node: f.Erl_client_name},
		etf.Tuple{etf.Atom("fl_client_ready"), f.Process.Info().PID},
	)

	log.Printf("after sending (%s, %s)! (fl_client_ready)", f.Erl_worker_mailbox, f.Erl_client_name)
}
func (f *FCMeansClient) process_client() {
}
func (f *FCMeansClient) destroy() {
	log.Printf("DESTROYYYY")
	os.Exit(0)
}

func main() {

	go_node_id := os.Args[1]         // go_c0ecdfb7-00f1-4270-8e46-d835bd00f153@127.0.0.1
	erl_client_name := os.Args[2]    // director@127.0.0.1
	erl_worker_mailbox := os.Args[3] // mboxserver_c0ecdfb7-00f1-4270-8e46-d835bd00f153
	erl_cookie := os.Args[4]         // cookie_123456789
	experiment_id := os.Args[5]      // c0ecdfb7-00f1-4270-8e46-d835bd00f153

	log.Printf("gorlang_node_id = %v, erl_client_name = %v, erl_worker_mailbox = %v, erl_cookie = %v, experiment_id = %v\n", go_node_id, erl_client_name, erl_worker_mailbox, erl_cookie, experiment_id)

	logFileName := os.Getenv("FL_CLIENT_LOG_FOLDER") + "/" + os.Getenv("FL_CLIENT_ID") + ".log"
	if logFileName == "" {
		logFileName = "default_client.log"
	}

	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(os.Stdout)

	node, err := ergo.StartNode(go_node_id, erl_cookie, node.Options{})
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}

	fcmeansclient := FCMeansClient{
		FedLangProcess: &common.FedLangProcess{
			Erl_client_name:    erl_client_name,
			Erl_worker_mailbox: erl_worker_mailbox,
		},
	}
	fcmeansclient.FedLangProcess.Callable = &fcmeansclient

	fcmeansclient.Process, err = node.Spawn(experiment_id, gen.ProcessOptions{}, fcmeansclient.FedLangProcess)
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}

	err = fcmeansclient.Process.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("node_ready"), fcmeansclient.Process.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
	log.Printf("fcmeansclient terminated\n")

}