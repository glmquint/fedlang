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
	"strconv"
	"time"

	"github.com/ergo-services/ergo/etf"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"gonum.org/v1/gonum/mat"
)

var CLIENT_ID string

type FCMeansClient struct {
	factor_lambda   float64
	min_num_clients int
	target_feature  int
	X               [][]float64
	y_true          []float64
	rows            int
	num_features    int
	datasetName     string
	savedMetrics    []byte
	receivedData    [][]byte
	round_number    int
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
	f.receivedData = make([][]byte, 0)
	var experimentConfig ExperimentConfig
	err := json.Unmarshal(json_str_config, &experimentConfig)
	if err != nil {
		log.Fatalf("Error parsing JSON config: %v", err)
	}

	log.Printf("experiment = %s, experimentConfig = %+v\n", experiment, experimentConfig)

	f.factor_lambda = experimentConfig.LambdaFactor
	f.min_num_clients = experimentConfig.NumClients
	f.target_feature = experimentConfig.TargetFeature
	f.datasetName = experimentConfig.DatasetName

	f.X, f.y_true, f.target_feature = load_experiment_info(f.min_num_clients, f.target_feature, f.datasetName)
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

	log.Printf("start process_client, expertiment = %v, round_number = %v, centers_param_len = %v\n", expertiment, round_number, len(centers_param))
	var centers_list [][]float64
	// var decodedResult_tmp interface{}
	err := decodeFromBytes(centers_param, &centers_list)
	if err != nil {
		log.Println("Error decoding:", err)
		panic(err)
	}
	// for _, v := range decodedResult_tmp.([]interface{}) {
	// 	arr := make([]float64, 0)
	// 	for _, vv := range v.([]interface{}) {
	// 		arr = append(arr, vv.(float64))
	// 	}
	// 	centers_list = append(centers_list, arr)
	// }
	// log.Printf("centers_list = %v\n", centers_list)

	// Convert centers to a matrix
	centers := mat.NewDense(len(centers_list), len(centers_list[0]), nil)
	for i := 0; i < len(centers_list); i++ {
		centers.SetRow(i, centers_list[i])
	}

	// Initialize variables
	numClusters := centers.ColView(0).Len() // TODO: check if we numCluster is over rows instead of columns
	numObjects := len(f.X)                  //pls use more meaningful names
	factorLambda := f.factor_lambda
	numFeatures := f.num_features

	// Initialize ws and u slices
	ws := mat.NewDense(numClusters, numFeatures, nil)
	u := mat.NewVecDense(numClusters, nil)

	for i := 0; i < numObjects; i++ {
		denom := 0.0
		numer := make([]float64, numClusters)
		x := f.X[i]
		for j := 0; j < numClusters; j++ {
			vc := centers.RawRowView(j)
			numer[j] = math.Pow(distance_fn([][]float64{x, vc}), (2 / (factorLambda - 1)))
			if numer[j] == 0 {
				numer[j] = 1e-10 // TODO: this is not the correct translation check it
			}
			denom += (1 + numer[j])
		}
		for j := 0; j < numClusters; j++ {
			u_c_i := math.Pow((numer[j] * denom), -1)
			//ws[c] = ws[c] + (u_c_i ** factor_lambda) * x
			for k := 0; k < numFeatures; k++ {
				// the illness of this single line of code is beyond me
				// set the element at row j and column k to the sum of the
				// element at row j and column k and the product of the
				// element x[k] and the power of u_c_i to factorLambda
				// also does not return anything the panic is inside the function
				ws.Set(j, k, ws.At(j, k)+(math.Pow(u_c_i, factorLambda)*x[k]))
			}
			//u[c] = u[c] + (u_c_i ** factor_lambda)
			// Similar to the above line, set the element at index j to the
			// sum of the element at index j and the power of u_c_i to factorLambda
			u.SetVec(j, u.AtVec(j)+math.Pow(u_c_i, factorLambda))
		}
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

	for i := 0; i < numObjects; i++ {
		denom := 0.0
		numer := make([]float64, numObjects)
		x := f.X[i]
		for j := 0; j < numClusters; j++ {
			vc := centers.RawRowView(j)
			numer[j] = math.Pow(distance_fn([][]float64{x, vc}), (2 / (factorLambda - 1)))
			if numer[j] == 0 {
				numer[j] = 1e-10 // TODO: his is not the correct translation check it
			}
			denom += (1 / numer[j])
		}
		for j := 0; j < numClusters; j++ {
			u_c_i := math.Pow((numer[j] * denom), -1)
			uMatrix[j][i] = u_c_i
		}
	}
	// u = np.asarray(u).T
	uMatrixT := make([][]float64, numObjects)
	for i := range uMatrixT {
		uMatrixT[i] = make([]float64, numClusters)
	}
	//y_pred = np.argmax(u, 1)
	y_pred := make([]float64, numObjects)
	for i := 0; i < numObjects; i++ {
		max := 0.0
		for j := 0; j < numClusters; j++ {
			if uMatrix[j][i] > max {
				max = uMatrix[j][i]
				y_pred[i] = float64(j)
			}
		}
	}
	// create a stub ARI score
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
	// s.Process.Send(
	// 	gen.ProcessID{Name: s.Erl_worker_mailbox, Node: s.Erl_client_name},
	// 	etf.Tuple{etf.Atom("process_server_ok"), buffer.Bytes(), metricsMessageBytes},
	// )
	log.Printf("end process_client, data_len = %v, metricsMessage = %v\n", len(data), metricsMessage)

	f.round_number = round_number
	fp.PeerSend(f.ringFunction(), "RingForward", data_bytes)
	// fp.PeerSend(f.ringFunction(), "RingForward", []byte("hello from "+CLIENT_ID))
	id, _ := strconv.Atoi(CLIENT_ID)
	if id != (round_number % 3) { // TODO: remove hardcoded number of clients
		return etf.Tuple{etf.Atom("fl_py_result_ack"), metricsMessageBytes}
	}
	f.savedMetrics = metricsMessageBytes
	return nil
}

func (f *FCMeansClient) ringFunction() func(int, int) int {
	rn := f.round_number
	return func(id, num_peers int) int {
		log.Printf("ringFunction: id = %d, num_peers = %d rn = %d\n", id, num_peers, rn)
		if id == (rn % num_peers) {
			log.Println("ringFunction: self send")
			return id
		}

		peer_id := (id + 1) % num_peers
		log.Printf("ringFunction: peer_id = %d\n", peer_id)
		return peer_id
	}
}

func (f *FCMeansClient) RingForward(data_bytes interface{}, fp common.FedLangProcess) {
	log.Printf("RingForward: received msg = %#v\n", data_bytes)
	id, _ := strconv.Atoi(CLIENT_ID)
	log.Printf("RingForward: id = %d, round_number = %d num_clients = %d\n", id, f.round_number, fp.NumClients)
	if id != (f.round_number % fp.NumClients) {
		log.Printf("RingForward: not my turn, forwarding\n")
		fp.PeerSend(f.ringFunction(), "RingForward", data_bytes)
		return
	}
	// master peer should aggregate the results
	// TODO: for now it just sends all the results once they all arrive
	f.receivedData = append(f.receivedData, data_bytes.([]byte))
	log.Printf("RingForward: received data len = %d\n", len(f.receivedData))
	if fp.NumClients == 0 || len(f.receivedData) < fp.NumClients {
		return
	}
	log.Printf("RingForward: all results have arrived\n")
	// all results have arrived
	for _, data := range f.receivedData {
		// process the data
		fp.WorkerSend(etf.Tuple{etf.Atom("fl_py_result"), data, f.savedMetrics})
		// break // for now, just process the first result TODO: aggregate the results
	}
	f.receivedData = make([][]byte, 0)
}

func (f *FCMeansClient) Destroy(fp common.FedLangProcess) {
	log.Printf("DESTROYYYY")
	os.Exit(0)
}

// func (f *FCMeansClient) Update_graph(clients etf.Term, fp common.FedLangProcess) {
// 	log.Printf("Update_graph")
// 	log.Printf("clients = %#v\n", clients)
// }

func main() {

	go_node_id := os.Args[1]         // go_c0ecdfb7-00f1-4270-8e46-d835bd00f153@127.0.0.1
	erl_client_name := os.Args[2]    // director@127.0.0.1
	erl_worker_mailbox := os.Args[3] // mboxserver_c0ecdfb7-00f1-4270-8e46-d835bd00f153
	erl_cookie := os.Args[4]         // cookie_123456789
	experiment_id := os.Args[5]      // c0ecdfb7-00f1-4270-8e46-d835bd00f153
	CLIENT_ID = os.Args[6]           // 0

	log.Printf("gorlang_node_id = %v, erl_client_name = %v, erl_worker_mailbox = %v, erl_cookie = %v, experiment_id = %v\n", go_node_id, erl_client_name, erl_worker_mailbox, erl_cookie, experiment_id)

	logFileName := os.Getenv("FL_CLIENT_LOG_FOLDER") + "/" + CLIENT_ID + ".log"
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
