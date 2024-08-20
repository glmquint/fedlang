package main

import (
	"bytes"
	"log"
	"math"
	"os"
	"time"

	"encoding/json"
	"errors"
	"math/rand"
	"reflect"

	"fcmeans/common"
	"github.com/MacIt/pickle"
	"github.com/ergo-services/ergo"
	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/ergo-services/ergo/node"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"gonum.org/v1/gonum/mat"
)

type FCMeansServer struct {
	*common.FedLangProcess
	target_feature    int
	max_number_rounds int
	num_clusters      int
	epsilon           int
	num_features      int
	cluster_centers   [][][]float64
	*FLExperiment
	currentRound int
}

func (s *FCMeansServer) Call(funcName string, params ...interface{}) (result interface{}, err error) {
	StubStorage := map[string]interface{}{
		"init_server":    s.init_server,
		"start_round":    s.start_round,
		"process_server": s.process_server,
		"finish":         s.finish,
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

func (s *FCMeansServer) init_server(experiment, json_str_config, bb string) {
	byteSlice := []byte(bb)
	client_ids := make([]int, len(byteSlice))
	for i, b := range byteSlice {
		client_ids[i] = int(b)
	}
	var experiment_config map[string]interface{}

	// Parse the JSON string
	err := json.Unmarshal([]byte(json_str_config), &experiment_config)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("experiment = %#v, client_ids = %#v, experiment_config = %#v\n", experiment, client_ids, experiment_config)
	parameters := experiment_config["parameters"].(map[string]interface{})
	log.Printf("paramters->numFeatures = %#v\n", parameters["numFeatures"])
	n_features := int(parameters["numFeatures"].(float64))
	seed := int(parameters["seed"].(float64))
	n_clusters := int(parameters["numClusters"].(float64))
	s.target_feature = int(parameters["targetFeature"].(float64))
	s.max_number_rounds = int(experiment_config["maxNumberOfRounds"].(float64)) // WARN: max_number_rounds should be in parameters??
	lambda_factor := int(parameters["lambdaFactor"].(float64))
	stop_condition_threshold := experiment_config["stopConditionThreshold"]
	log.Printf("n_features = %#v, seed = %#v, n_clusters = %#v, target_feature = %#v, max_number_rounds = %#v, lambda_factor = %#v, stop_condition_threshold = %#v\n", n_features, seed, n_clusters, s.target_feature, s.max_number_rounds, lambda_factor, stop_condition_threshold)
	if stop_condition_threshold != nil {
		s.epsilon = int(stop_condition_threshold.(float64))
	} else {
		s.epsilon = -1 // HACK: check logic behind stop_condition using epsilon
		log.Println("stop_condition_threshold is nil")
	}
	log.Printf("n_clusters = %#v, epsilon = %#v, max_number_rounds = %#v, n_features = %#v", n_clusters, s.epsilon, s.max_number_rounds, n_features)
	min_num_clients := int(experiment_config["minNumberClients"].(float64))
	log.Printf("min_num_clients = %#v\n", min_num_clients)
	flexperiment := FLExperiment{
		_max_number_rounds: s.max_number_rounds,
		_client_ids:        client_ids,
		_client_configuration: map[string]interface{}{
			"lambdaFactor":  lambda_factor,
			"numClients":    min_num_clients,
			"targetFeature": s.target_feature,
			"do_sleep":      true,
		},
		_latency_required: true,
		_calls_list:       nil,
		_validate:         false,
	}
	log.Printf("flexperiment = %#v\n", flexperiment)
	rand.Seed(int64(seed))

	centers := make([][]float64, n_clusters)
	for i := 0; i < n_clusters; i++ {
		centers[i] = make([]float64, n_features)
		for j := 0; j < n_features; j++ {
			centers[i][j] = rand.Float64()
		}
	}
	// log.Printf("centers = %#v\n", centers)
	s.num_clusters = n_clusters
	s.num_features = n_features
	cluster_centers := make([][][]float64, 0)
	cluster_centers = append(cluster_centers, centers)
	s.cluster_centers = cluster_centers
	_ = make([]float64, 0)

	centerList := make([][]float64, len(centers))
	for i, center := range centers {
		centerList[i] = center
	}
	log.Printf("centerList = %#v\n", centerList)

	flexperiment._global_model_parameters = centerList

	flexperiment._type_of_termination = "custom"
	flexperiment._type_of_client_selection = "probability"
	flexperiment._client_ratio = 1
	flexperiment._client_values = nil
	flexperiment._step_wise_client_selection = false
	flexperiment._client_selection_threshold = nil
	s.FLExperiment = &flexperiment
	s.currentRound = 0

	clientConfig, _, _, _, _, callsList := flexperiment.get_initialization()
	log.Printf("clientConfig = %#v\n", clientConfig)

	clientConfigurationStr, err := json.Marshal(clientConfig)
	if err != nil {
		log.Fatalf("Error marshaling client config: %v", err)
	}
	atom := etf.Atom("fl_server_ready")
	log.Printf("atom = %#v\n", atom)
	pid := s.Process.Info().PID
	log.Printf("pid = %#v\n", pid)
	// log.Printf("clientConfigurationStr = %#v\n", clientConfigurationStr)
	log.Printf("callsList = %#v\n", callsList)
	msg := etf.Tuple{atom, pid, clientConfigurationStr, callsList}
	log.Printf("sending message = %#v\n", msg)
	err = s.Process.Send(
		gen.ProcessID{Name: s.Erl_worker_mailbox, Node: s.Erl_client_name},
		msg,
	)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
		panic(err)
	}
	log.Printf("message sent = %#v\n", msg)
}

func (e *FLExperiment) get_initialization() (map[string]interface{}, int, []byte, float64, int, []etf.Tuple) {
	typeOfTermination := []byte(e._type_of_termination)

	// Handling the callsList logic
	if len(e._calls_list) == 0 {
		if e._validate {
			e._calls_list = append(e._calls_list, etf.Tuple{[]byte("standard"), []byte("two_step")})
		} else {
			e._calls_list = append(e._calls_list, etf.Tuple{[]byte("standard"), []byte("one_step")})
		}
	}

	// Returning multiple values
	return e._client_configuration, e._max_number_rounds, typeOfTermination, e._termination_threshold, e._latency, e._calls_list
}

var startFlTime = time.Time{}

func (s *FCMeansServer) start_round(round_mail_box, experiment string, round_number int) {
	// log.Printf("round_mail_box = %#v, experiment = %#v, round_number = %#v\n", round_mail_box, experiment, round_number)
	if startFlTime == (time.Time{}) {
		startFlTime = time.Now()
	}
	log.Printf("start_fl_time = %#v\n", startFlTime)
	result := s.FLExperiment._global_model_parameters
	// log.Printf("start round result = %#v\n", result)
	client_ids := s.FLExperiment._client_ids
	log.Printf("before sending result to (%#v, %#v)", s.Erl_worker_mailbox, s.Erl_client_name)

	// Create a new encoder and encode the result
	var buffer bytes.Buffer
	encoder := pickle.NewEncoder(&buffer)
	if err := encoder.Encode(result); err != nil {
		log.Fatal("Error encoding:", err)
		panic(err)
	}

	// The serialized data is now in buffer.Bytes()
	tt := buffer.Bytes()
	// log.Println("Serialized data:", tt)

	//--------------------------------------
	var decodedResult [][]float64
	decoder := pickle.NewDecoder(bytes.NewReader(tt))
	decodedResult_tmp, err := decoder.Decode()
	if err != nil {
		log.Println("Error decoding:", err)
		panic(err)
	}
	for _, v := range decodedResult_tmp.([]interface{}) {
		arr := make([]float64, 0)
		for _, vv := range v.([]interface{}) {
			arr = append(arr, vv.(float64))
		}
		decodedResult = append(decodedResult, arr)
	}

	// log.Println("Deserialized data:", decodedResult)

	//--------------------------------------

	err = s.Process.Send(
		gen.ProcessID{Name: round_mail_box, Node: s.Erl_client_name},
		etf.Tuple{etf.Atom("start_round_ok"), tt, client_ids},
	)
	if err != nil {
		panic(err)
	}

	log.Printf("after sending (%#v, %#v) ! (start_round_ok)", s.Erl_worker_mailbox, s.Erl_client_name)
}

func (fl *FLExperiment) get_step_data() (interface{}, []int) {
	var step_selected_ids []int
	if !fl._step_wise_client_selection {
		step_selected_ids = fl._round_selected_ids
	} else {
		// TODO: implement perform_client_selection
		//step_selected_ids = fl._perform_client_selection()
		panic("Not implemented")
	}
	fl._round_selected_ids = step_selected_ids
	return fl._global_model_parameters, fl._round_selected_ids
}
func flatten(input [][]float64) []float64 {
	var result []float64
	for _, innerSlice := range input {
		result = append(result, innerSlice...)
	}
	return result
}
func (s *FCMeansServer) process_server(round_mail_box string, experiment string, config_file int, client_responses etf.List) {
	log.Printf("Starting process_server ...")
	// log.Printf("round_mail_box = %#v, experiment = %#v, config_file = %#v, client_responses = %#v\n", round_mail_box, experiment, config_file, client_responses)

	rand.Seed(time.Now().UnixNano())
	const low = 2
	const high = 4
	// time_to_sleep := low + rand.Float64()*(high-low)
	// time.Sleep(time.Duration(time_to_sleep) * time.Second)

	// log.Printf("start process_server, experiment = %s, round_mail_box = %s, len(client_responses) = %d\n", experiment, round_mail_box, len(client_responses))

	type clientDataType struct {
		clientId int
		result   pickle.Tuple
	}
	var data []clientDataType
	// etf.List{etf.Tuple{0, []uint8{
	for _, clientResponse := range client_responses {
		clientResponseSlice, ok := clientResponse.(etf.Tuple)
		if !ok {
			panic("Error: clientResponse is not of type etf.Tuple")
		}
		if len(clientResponseSlice) < 2 {
			panic("Error: clientResponseSlice is < 2")
		}
		var decodedResult pickle.Tuple
		// ([1.0, 2.0, ...], [[1.0, 2.0, ...], [1.0, 2.0, ...], ...])
		decoder := pickle.NewDecoder(bytes.NewReader(clientResponseSlice[1].([]byte)))
		decodedResult_tmp, err := decoder.Decode()
		if err != nil {
			panic(err)
		}

		decodedResult, ok = decodedResult_tmp.(pickle.Tuple)
		if !ok {
			panic("Error: decodedResult is not of type resultType")
		}

		responseData := clientDataType{clientResponseSlice[0].(int), decodedResult}
		data = append(data, responseData)
	}

	for i, d := range data {
		log.Printf("i = %d, data[0] = %v", i, d.clientId)
	}

	var cl_resp []pickle.Tuple
	for _, cr := range data {
		cl_resp = append(cl_resp, cr.result)
	}
	// log.Printf("cl_resp = %#v\n", cl_resp)

	s.currentRound += 1
	num_clients := len(client_responses)
	log.Printf("num_clients = %#v\n", num_clients)

	uList := make([]float64, s.num_clusters)
	wsList := make([][]float64, s.num_clusters)
	for i := range wsList {
		wsList[i] = make([]float64, s.num_features)
	}

	for client_idx := 0; client_idx < num_clients; client_idx++ {
		response := data[client_idx].result
		us := response[0].([]interface{})
		wss := response[1].([]interface{})
		for i := 0; i < s.num_clusters; i++ {
			var clientWs *mat.Dense
			clientU := us[i].(float64)
			// log.Printf("clientU = %#v\n", clientU)
			ws := wss[i].([]interface{})
			// log.Printf("ws = %#v\n", ws)
			clientWs = mat.NewDense(1, len(ws), nil)
			for j, w := range ws {
				clientWs.Set(0, j, w.(float64))
			}
			// log.Printf("clientWs = %#v\n", clientWs)
			uList[i] += clientU
			for j := 0; j < clientWs.RawMatrix().Cols; j++ {
				wsList[i][j] += clientWs.At(0, j)
			}
		}
	}
	var newClusterCenters [][]float64
	prevClusterCenters := s.cluster_centers[len(s.cluster_centers)-1]

	for i := 0; i < s.num_clusters; i++ {
		u := uList[i]
		ws := wsList[i]
		var center []float64
		if u == 0 {
			center = prevClusterCenters[i]
		} else {
			center = make([]float64, len(ws))
			for j := range ws {
				center[j] = ws[j] / float64(u)
			}
		}
		newClusterCenters = append(newClusterCenters, center)
	}
	s.FLExperiment._global_model_parameters = newClusterCenters
	s.cluster_centers = append(s.cluster_centers, newClusterCenters)

	step_data, _ := s.FLExperiment.get_step_data() // TODO add client ids
	centersR := mat.NewDense(len(newClusterCenters), len(newClusterCenters[0]), flatten(newClusterCenters))
	centersR1 := mat.NewDense(len(prevClusterCenters), len(prevClusterCenters[0]), flatten(prevClusterCenters))
	diffMatrix := mat.NewDense(centersR.RawMatrix().Rows, centersR.RawMatrix().Cols, nil)
	diffMatrix.Sub(centersR, centersR1)

	v, _ := mem.VirtualMemory()
	total_memory := v.Total
	used_memory := v.Used
	memory_usage_percentage := math.Round((float64(used_memory)/float64(total_memory))*100) / 100

	cpu_percentages, _ := cpu.Percent(time.Second, false)

	metricsMessage := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"round":     s.currentRound,
		"hostMetrics": map[string]float64{
			"cpuUsagePercentage":    cpu_percentages[0],
			"memoryUsagePercentage": memory_usage_percentage,
		},
		"modelMetrics": map[string](mat.Matrix){
			"FRO": diffMatrix,
		},
	}
	var buffer bytes.Buffer

	// Create a new encoder and encode the result
	encoder := pickle.NewEncoder(&buffer)
	if err := encoder.Encode(step_data); err != nil {
		panic(err)
	}
	metricsMessageBytes, _ := json.Marshal(metricsMessage)
	s.Process.Send(
		gen.ProcessID{Name: s.Erl_worker_mailbox, Node: s.Erl_client_name},
		etf.Tuple{etf.Atom("process_server_ok"), buffer.Bytes(), metricsMessageBytes},
	)

}

func (s *FCMeansServer) finish() {
	log.Printf("DESTROY")
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
		logFileName = "default.log"
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

	fcmeansserver := FCMeansServer{
		FedLangProcess: &common.FedLangProcess{
			Erl_client_name:    erl_client_name,
			Erl_worker_mailbox: erl_worker_mailbox,
		},
	}
	fcmeansserver.FedLangProcess.Callable = &fcmeansserver

	fcmeansserver.Process, err = node.Spawn(experiment_id, gen.ProcessOptions{}, fcmeansserver.FedLangProcess)
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}

	err = fcmeansserver.Process.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("node_ready"), fcmeansserver.Process.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
	log.Printf("FCMeansServer terminated\n")
}
