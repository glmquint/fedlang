package main

import (
	"encoding/csv"
	"encoding/json"
	"fcmeans/common"
	"github.com/ergo-services/ergo/etf"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
)

type FCMeansClient struct {
	factor_lambda  float64
	num_clients    int
	target_feature int
	X              [][]float64
	y_true         []float64
	rows           int
	num_features   int
	datasetName    string
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
	TargetFeature int     `json:"targetFeature"`
	DatasetName   string  `json:"datasetName"`
}

func distance_fn(A [][]float64) float64 {
	var sum float64
	for i := range A {
		for j := range A[i] {
			sum += A[i][j] * A[i][j]
		}
	}
	return math.Sqrt(sum)
}

func load_experiment_info(numClients int, targetFeature int, datasetName ...string) ([][]float64, []float64, int) {
	basePath := filepath.Join(os.Getenv("PROJECT_PATH"), "datasets")
	var datasetPath string
	//log.Printf("datasetName = %#v len = %d", datasetName, len(datasetName))
	if len(datasetName) <= 1 {
		datasetPath = filepath.Join(basePath, "pendigits.csv")
	} else {
		datasetPath = filepath.Join(basePath, datasetName[0])
	}
	log.Printf("dataset path = %s\n", datasetPath)
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
	log.Printf("dataset chunk size = %d\n", size)

	header := records[0]
	targetIndex := -1
	for i, col := range header {
		if col == strconv.Itoa(targetFeature) {
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

	log.Printf("dataset chunk size = %d, X.shape = (%d, %d), y.shape = (%d)\n", size, len(dfX), len(dfX[0]), len(yTrueSample))
	return values, yTrueSample, targetFeature
}
func (f *FCMeansClient) Init_client(experiment string, json_str_config []byte, fp common.FedLangProcess) etf.Term {
	//log.Printf("experiment = %#v json_str_config = %#v\n", experiment, json_str_config)
	var experimentConfig ExperimentConfig
	err := json.Unmarshal(json_str_config, &experimentConfig)
	if err != nil {
		log.Fatalf("Error parsing JSON config: %v", err)
	}

	log.Printf("experiment = %s, experimentConfig = %+v\n", experiment, experimentConfig)

	f.factor_lambda = experimentConfig.LambdaFactor
	f.num_clients = experimentConfig.NumClients
	f.target_feature = experimentConfig.TargetFeature
	f.datasetName = experimentConfig.DatasetName

	f.X, f.y_true, f.target_feature = load_experiment_info(f.num_clients, f.target_feature, f.datasetName)
	f.rows, f.num_features = len(f.X), len(f.X[0])
	/*
		f.Process.Send(
			gen.ProcessID{Name: f.Erl_worker_mailbox, Node: f.Erl_client_name},
			etf.Tuple{etf.Atom("fl_client_ready"), f.Process.Info().PID},
		)
	*/
	return etf.Tuple{etf.Atom("fl_client_ready"), fp.Process.Info().PID}
}
func (f *FCMeansClient) Process_client(fp common.FedLangProcess) {
}
func (f *FCMeansClient) Destroy(fp common.FedLangProcess) {
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

	common.StartProcess[FCMeansClient](go_node_id, erl_cookie, erl_client_name, erl_worker_mailbox, experiment_id)
}
