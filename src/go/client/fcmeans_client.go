package main

import (
	"bytes"
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"fcmeans/common"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/ergo-services/ergo/etf"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"gonum.org/v1/gonum/mat"
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
	savedMetrics   []byte
	receivedData   []interface{}
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

func encodeToBytes(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeFromBytes(data []byte, v interface{}) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	return decoder.Decode(v)
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

	random_gen := rand.New(rand.NewSource(42))
	randomSamples := random_gen.Perm(len(dfX))[:size]
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
	f.receivedData = make([]interface{}, 0)
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
	gob.Register([][]float64{})

	return etf.Tuple{etf.Atom("fl_client_ready"), fp.Own_pid}
}

func (f *FCMeansClient) Process_client(expertiment string, round_number int, centers_param []byte, fp common.FedLangProcess) etf.Term {
	log.Printf("start process_client, expertiment = %v, round_number = %v, centers_param = %v\n", expertiment, round_number, centers_param)

	// Decode centers_param into centers_list
	var centers_list [][]float64
	err := decodeFromBytes(centers_param, &centers_list)
	if err != nil {
		log.Println("Error decoding:", err)
		panic(err)
	}

	// Convert centers to a matrix
	centers := mat.NewDense(len(centers_list), len(centers_list[0]), nil)
	for i := 0; i < len(centers_list); i++ {
		centers.SetRow(i, centers_list[i])
	}

	// Initialize variables
	numClusters := centers.ColView(0).Len()
	numObjects := len(f.X)
	factorLambda := f.factor_lambda
	numFeatures := f.num_features

	// Initialize ws and u slices
	ws := mat.NewDense(numClusters, numFeatures, nil)
	u := mat.NewVecDense(numClusters, nil)

	// Channel to collect results
	wsCh := make(chan struct {
		i     int
		wsRow []float64
		uVal  float64
	}, numObjects)

	// Goroutines to calculate ws and u
	var wg sync.WaitGroup
	for i := 0; i < numObjects; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			denom := 0.0
			numer := make([]float64, numClusters)
			x := f.X[i]
			for j := 0; j < numClusters; j++ {
				vc := centers.RawRowView(j)
				numer[j] = math.Pow(distance_fn([][]float64{x, vc}), (2 / (factorLambda - 1)))
				if numer[j] == 0 {
					numer[j] = 1e-10
				}
				denom += (1 + numer[j])
			}

			wsRow := make([]float64, numFeatures)
			for j := 0; j < numClusters; j++ {
				u_c_i := math.Pow((numer[j] * denom), -1)
				for k := 0; k < numFeatures; k++ {
					wsRow[k] += math.Pow(u_c_i, factorLambda) * x[k]
				}
				uVal := u.AtVec(j) + math.Pow(u_c_i, factorLambda)
				wsCh <- struct {
					i     int
					wsRow []float64
					uVal  float64
				}{i: j, wsRow: wsRow, uVal: uVal}
			}
		}(i)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(wsCh)
	}()

	for result := range wsCh {
		ws.SetRow(result.i, result.wsRow)
		u.SetVec(result.i, result.uVal)
	}

	// Convert ws to a slice of slices and store data in a tuple
	wsSlice := make([][]float64, numClusters)
	for i := 0; i < numClusters; i++ {
		wsSlice[i] = ws.RawRowView(i)
	}
	data := []interface{}{
		u.RawVector().Data,
		wsSlice,
	}

	// Reinitialize u to a 2D slice with zeros
	uMatrix := make([][]float64, numClusters)
	for i := range uMatrix {
		uMatrix[i] = make([]float64, numObjects)
	}

	// Channel to collect uMatrix results
	uMatrixCh := make(chan struct {
		i    int
		uRow []float64
	}, numObjects)

	// Goroutines to calculate uMatrix
	for i := 0; i < numObjects; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			denom := 0.0
			numer := make([]float64, numClusters)
			x := f.X[i]
			for j := 0; j < numClusters; j++ {
				vc := centers.RawRowView(j)
				numer[j] = math.Pow(distance_fn([][]float64{x, vc}), (2 / (factorLambda - 1)))
				if numer[j] == 0 {
					numer[j] = 1e-10
				}
				denom += (1 / numer[j])
			}

			uRow := make([]float64, numClusters)
			for j := 0; j < numClusters; j++ {
				u_c_i := math.Pow((numer[j] * denom), -1)
				uRow[j] = u_c_i
			}
			uMatrixCh <- struct {
				i    int
				uRow []float64
			}{i: i, uRow: uRow}
		}(i)
	}

	// Collect uMatrix results
	go func() {
		wg.Wait()
		close(uMatrixCh)
	}()

	for result := range uMatrixCh {
		for j := 0; j < numClusters; j++ {
			uMatrix[j][result.i] = result.uRow[j]
		}
	}

	// Transpose uMatrix and calculate y_pred
	uMatrixT := make([][]float64, numObjects)
	y_pred := make([]float64, numObjects)

	for i := 0; i < numObjects; i++ {
		uMatrixT[i] = make([]float64, numClusters)
		max := 0.0
		for j := 0; j < numClusters; j++ {
			uMatrixT[i][j] = uMatrix[j][i]
			if uMatrix[j][i] > max {
				max = uMatrix[j][i]
				y_pred[i] = float64(j)
			}
		}
	}

	// Create a stub ARI score and metrics message
	ari_client := 0
	v, _ := mem.VirtualMemory()
	total_memory := v.Total
	used_memory := v.Used
	memory_usage_percentage := math.Round((float64(used_memory)/float64(total_memory))*100) / 100

	cpu_percentages, _ := cpu.Percent(time.Second, false)

	metricsMessage := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"round":     round_number,
		"clientId":  fp.Own_pid,
		"hostMetrics": map[string]float64{
			"cpuUsagePercentage":    cpu_percentages[0],
			"memoryUsagePercentage": memory_usage_percentage,
		},
		"modelMetrics": map[string](float64){
			"ARI": float64(ari_client),
		},
	}
	data_bytes, err := encodeToBytes(data)
	if err != nil {
		panic(err)
	}
	metricsMessageBytes, _ := json.Marshal(metricsMessage)

	log.Printf("end process_client, data = %v, metricsMessage = %v\n", data, metricsMessage)

	fp.PeerSend(func(id, num_peers int) int { return max((id - 1), 0) }, "RingForward", data_bytes)
	if os.Getenv("FL_CLIENT_ID") != "0" {
		return etf.Tuple{etf.Atom("fl_py_result_ack"), metricsMessageBytes}
	}
	f.savedMetrics = metricsMessageBytes
	return nil
}

func (f *FCMeansClient) RingForward(data_bytes interface{}, fp common.FedLangProcess) {
	log.Printf("RingForward: received msg = %s\n", data_bytes)
	if os.Getenv("FL_CLIENT_ID") != "0" {
		fp.PeerSend(func(id, num_peers int) int { return max((id - 1), 0) }, "RingForward", data_bytes)
		return
	}
	// master peer should aggregate the results
	// TODO: for now it just sends all the results once they all arrive
	f.receivedData = append(f.receivedData, data_bytes)
	if fp.NumClients == 0 || len(f.receivedData) < fp.NumClients {
		return
	}
	// all results have arrived
	for data := range f.receivedData {
		// process the data
		fp.WorkerSend(etf.Tuple{etf.Atom("fl_py_result"), data, f.savedMetrics})
	}
	f.receivedData = make([]interface{}, 0)
}

func (f *FCMeansClient) Destroy(fp common.FedLangProcess) {
	log.Printf("DESTROYYYY")
	pprof.StopCPUProfile()
	cpufile.Close()
	os.Exit(0)
}

//	func (f *FCMeansClient) Update_graph(clients etf.Term, fp common.FedLangProcess) {
//		log.Printf("Update_graph")
//		log.Printf("clients = %#v\n", clients)
//	}
var cpufile *os.File

func main() {
	var err error
	cpufile, err = os.Create("client_cpu.prof")
	if err != nil {
		panic(err)
	}
	pprof.StartCPUProfile(cpufile)
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
	log.SetOutput(logFile)

	common.StartProcess[FCMeansClient](go_node_id, erl_cookie, erl_client_name, erl_worker_mailbox, experiment_id)
}
