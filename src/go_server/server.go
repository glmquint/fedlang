package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"reflect"

	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
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
		"init_server": s.init_server,
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
	_step_wise_client_selection interface{}
	_client_selection_threshold interface{}
	_round_selected_ids         interface{}
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
