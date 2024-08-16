package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"reflect"
	"time"

	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"gonum.org/v1/gonum/mat"
)

type gorlangServer struct {
	gen.Server
	proc               gen.Process
	erl_client_name    string
	erl_worker_mailbox string
	target_feature     int
	max_number_rounds  int
	num_clusters       int
	epsilon            int
	num_features       int
	cluster_centers    [][][]float64
	experiment         FLExperiment
	currentRound       int
}

func (s *gorlangServer) HandleCast(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	log.Printf("[%s] HandleCast: %#v\n", process.Name(), message)
	switch message {
	case etf.Atom("stop"):
		return gen.ServerStatusStopWithReason("stop they said")
	}
	return gen.ServerStatusOK
}

func (s *gorlangServer) HandleCall(process *gen.ServerProcess, from gen.ServerFrom, message etf.Term) (etf.Term, gen.ServerStatus) {
	log.Printf("[%s] HandleCall: %#v, From: %s\n", process.Name(), message, from.Pid)

	switch message.(type) {
	case etf.Atom:
		return "hello", gen.ServerStatusOK

	default:
		return message, gen.ServerStatusOK
	}
}

// HandleInfo
func (s *gorlangServer) HandleInfo(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	log.Printf("[%s] HandleInfo: %#v\n", process.Name(), message)
	switch message.(type) {
	case etf.Atom:
		switch message {
		case etf.Atom("ping"):
			err := process.Send(gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name}, "pong")
			if err != nil {
				panic(err)
			}
		}
	case etf.Tuple:
		message_as_tuple, ok := message.(etf.Tuple)
		if !ok {
			log.Println("error: cannot cast message to Tuple")
			return gen.ServerStatusIgnore
		}
		if len(message_as_tuple) < 2 {
			log.Println("error: message_as_tuple is < 2")
			return gen.ServerStatusIgnore
		}
		pid := message_as_tuple[0]
		fun_name := fmt.Sprintf("%v", message_as_tuple[1])
		if !ok {
			log.Println("error: cannot cast message_as_tuple to string")
			return gen.ServerStatusIgnore
		}
		args := message_as_tuple[2:]
		args_slice := make([]interface{}, len(args))
		for i, v := range args {
			args_slice[i] = v
		}
		log.Printf("sender = %#v, fun_name = %#v\n", pid, fun_name)
		Call(s, fun_name, args_slice...)
	}
	return gen.ServerStatusOK
}

func (s *gorlangServer) Terminate(process *gen.ServerProcess, reason string) {
	log.Printf("[%s] Terminating process with reason %q", process.Name(), reason)
}

func Call(s *gorlangServer, funcName string, params ...interface{}) (result interface{}, err error) {
	StubStorage := map[string]interface{}{
		"init_server":    s.init_server,
		"process_server": s.process_server,
	}

	log.Printf("funcname = %s, params = %v\n", funcName, params)

	f := reflect.ValueOf(StubStorage[funcName])
	if len(params) != f.Type().NumIn() {
		err = errors.New("The number of params is out of index.")
		return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		log.Printf("param[%d] = %#v\n", k, param)
		in[k] = reflect.ValueOf(param)
	}
	var res []reflect.Value
	log.Printf("Calling %#v with in_args %#v\n", f, in)
	res = f.Call(in)
	result = res[0].Interface()
	return
}

type FLExperiment struct {
	_global_model_parameters    interface{}
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
	_calls_list                 [][][]byte
	_validate                   bool
	experiment_history          interface{}
	_latency_required           bool
}

func (s *gorlangServer) init_server(experiment, json_str_config, bb string) {
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
	flexperiment := FLExperiment{
		_max_number_rounds: s.max_number_rounds,
		_client_ids:        client_ids,
		_client_configuration: map[string]interface{}{
			"_lambdaFactor":  lambda_factor,
			"_numClients":    min_num_clients,
			"_targetFeature": s.target_feature,
			"_do_sleep":      true,
		},
		_latency_required: true,
		_calls_list:       make([][][]byte, 0),
		_validate:         false,
	}
	rand.Seed(int64(seed))

	// Create centers matrix with random values
	centers := make([][]float64, n_clusters)
	for i := 0; i < n_clusters; i++ {
		centers[i] = make([]float64, n_features)
		for j := 0; j < n_features; j++ {
			centers[i][j] = rand.Float64()
		}
	}
	s.num_clusters = n_clusters
	s.num_features = n_features
	cluster_centers := make([][][]float64, 0)
	cluster_centers = append(cluster_centers, centers)
	s.cluster_centers = cluster_centers
	_ = make([]float64, 0)

	// Convert centers to a list of lists (slices of slices in Go)
	centerList := make([][]float64, len(centers))
	for i, center := range centers {
		centerList[i] = center
	}

	// Set global model parameters
	flexperiment._global_model_parameters = centerList

	// Set types of termination and client selection
	flexperiment._type_of_termination = "custom"
	flexperiment._type_of_client_selection = "probability"
	flexperiment._client_ratio = 1
	flexperiment._client_values = nil
	flexperiment._step_wise_client_selection = false
	flexperiment._client_selection_threshold = nil
	s.experiment = flexperiment
	s.currentRound = 0

	// Get initialization values
	clientConfig, _, _, _, _, callsList := flexperiment.get_initialization()

	// Convert client configuration to JSON string
	clientConfigurationStr, err := json.Marshal(clientConfig)
	if err != nil {
		log.Fatalf("Error marshaling client config: %v", err)
	}
	err = s.proc.Send(
		gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name},
		etf.Tuple{etf.Atom("fl_server_ready"), s.proc.Info().PID, clientConfigurationStr, callsList},
	)
	if err != nil {
		panic(err)
	}
}

func (e *FLExperiment) get_initialization() (map[string]interface{}, int, []byte, float64, int, [][][]byte) {
	typeOfTermination := []byte(e._type_of_termination)

	// Handling the callsList logic
	if len(e._calls_list) == 0 {
		if e._validate {
			e._calls_list = [][][]byte{
				{[]byte("standard"), []byte("two_step")},
			}
		} else {
			e._calls_list = [][][]byte{
				{[]byte("standard"), []byte("one_step")},
			}
		}
	}

	// Returning multiple values
	return e._client_configuration, e._max_number_rounds, typeOfTermination, e._termination_threshold, e._latency, e._calls_list
}
func (fl *FLExperiment) get_step_data() (interface{}, []int) {
	var step_selected_ids []int
	if !fl._step_wise_client_selection {
		step_selected_ids = fl._round_selected_ids
	} else {
		// TODO: implement perform_client_selection
		//step_selected_ids = fl._perform_client_selection()
	}
	fl._round_selected_ids = step_selected_ids
	return fl._global_model_parameters, fl._round_selected_ids
}
func flatten(input interface{}) []float64 {
	var result []float64
	flattenHelper(input, &result)
	return result
}
func flattenHelper(input interface{}, result *[]float64) {
	val := reflect.ValueOf(input)
	switch val.Kind() {
	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			flattenHelper(val.Index(i).Interface(), result)
		}
	case reflect.Float64:
		*result = append(*result, val.Interface().(float64))
	}
}
func (s *gorlangServer) process_server(round_mail_box string, experiment string, config_file string, client_responses []interface{}) {
	log.Printf("Starting process_server ...")
	log.Printf("self = %#v, round_mail_box = %#v, experiment = %#v, config_file = %#v, client_responses = %#v\n", round_mail_box, experiment, config_file, client_responses)

	rand.Seed(time.Now().UnixNano())
	const low = 2
	const high = 4
	time_to_sleep := low + rand.Float64()*(high-low)
	time.Sleep(time.Duration(time_to_sleep) * time.Second)

	log.Printf("start process_server, experiment = %s, round_mail_box = %s, len(client_responses) = %d\n", experiment, round_mail_box, len(client_responses))

	var data [][]interface{}
	for _, clientResponse := range client_responses {
		clientResponseSlice, ok := clientResponse.([]interface{})
		if !ok {
			log.Printf("Error: clientResponse is not of type []interface{}")
			continue
		}
		var decodedResult map[string]interface{}
		decoder := gob.NewDecoder(bytes.NewReader(clientResponseSlice[1].([]byte)))
		if err := decoder.Decode(&decodedResult); err != nil {
			fmt.Println("Error decoding:", err)
		}

		responseData := []interface{}{clientResponseSlice[0], decodedResult}
		data = append(data, responseData)
	}

	for i, d := range data {
		log.Printf("i = %d, data[0] = %v", i, d[0])
	}

	var cl_resp [][]interface{}
	for _, cr := range data {
		cl_resp = append(cl_resp, cr[1].([]interface{}))
	}

	s.currentRound += 1
	num_clients := len(client_responses)

	uList := make([]float64, s.num_clusters)
	wsList := make([][]float64, s.num_clusters)
	for i := range wsList {
		wsList[i] = make([]float64, s.num_features)
	}

	for client_idx := 0; client_idx < num_clients; client_idx++ {
		response := client_responses[client_idx]
		responseSlice, ok := response.([]interface{})
		if !ok {
			log.Printf("Error: response is not of type []interface{}")
			continue
		}
		for i := 0; i < s.num_clusters; i++ {
			var clientWs *mat.Dense
			clientUs, ok := responseSlice[0].([]interface{})
			if !ok {
				log.Printf("Error: responseSlice[0] is not of type []interface{}")
				continue
			}
			if ws, ok := responseSlice[i].(*mat.Dense); ok {
				clientWs = ws
			} else {
				// Assuming clientWsSlice[i] is a 2D slice of float64
				wsData, ok := responseSlice[i].([][]float64)
				if !ok {
					log.Printf("Error: clientWsSlice[%d] is not of type [][]float64", i)
					continue
				}
				clientWs = mat.NewDense(len(wsData), len(wsData[0]), nil)
				for rowIdx, row := range wsData {
					for colIdx, val := range row {
						clientWs.Set(rowIdx, colIdx, val)
					}
				}
			}
			clientU := clientUs[i].(float64)
			uList[i] += clientU
			for j := 0; j < clientWs.RawMatrix().Cols; j++ {
				wsList[i][j] += clientWs.At(0, j)
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
		s.experiment._global_model_parameters = newClusterCenters
		s.cluster_centers = append(s.cluster_centers, newClusterCenters)

		data, _ := s.experiment.get_step_data() // TODO add client ids
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
		encoder := gob.NewEncoder(&buffer)
		if err := encoder.Encode(data); err != nil {
			fmt.Println("Error encoding:", err)
		}
		metricsMessageBytes, _ := json.Marshal(metricsMessage)
		s.proc.Send(
			gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name},
			etf.Tuple{etf.Atom("process_server_ok"), buffer.Bytes(), metricsMessageBytes},
		)

	}
}
