package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"encoding/gob"
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"

	"github.com/ergo-services/ergo"
	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/ergo-services/ergo/node"
)

type FCMeansServer struct {
	*FedLangProcess
	target_feature    int
	max_number_rounds int
	num_clusters      int
	epsilon           int
	num_features      int
	cluster_centers   [][][]float64
	FLExperiment
	currentRound int
}

func (s FCMeansServer) Call(funcName string, params ...interface{}) (result interface{}, err error) {
	StubStorage := map[string]interface{}{
		"init_server": s.init_server,
		"start_round": s.start_round,
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
	if len(res) == 0 {
		err = nil
		return
	}
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
	log.Printf("centers = %#v\n", centers)
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
	s.FLExperiment = flexperiment
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
	log.Printf("clientConfigurationStr = %#v\n", clientConfigurationStr)
	log.Printf("callsList = %#v\n", callsList)
	msg := etf.Tuple{atom, pid, clientConfigurationStr, callsList}
	log.Printf("sending message = %#v\n", msg)
	err = s.Process.Send(
		gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name},
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
	log.Printf("round_mail_box = %#v, experiment = %#v, round_number = %#v\n", round_mail_box, experiment, round_number)
	if startFlTime == (time.Time{}) {
		startFlTime = time.Now()
	}
	log.Printf("start_fl_time = %#v\n", startFlTime)
	result := s.FLExperiment._global_model_parameters
	client_ids := s.FLExperiment._client_ids
	log.Printf("start round result = %#v\n", result)
	log.Printf("before sending result to [(%#v, %#v)", s.erl_worker_mailbox, s.erl_client_name)

	// Create a new encoder and encode the result
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err := encoder.Encode(result); err != nil {
		fmt.Println("Error encoding:", err)
	}

	// The serialized data is now in buffer.Bytes()
	tt := buffer.Bytes()
	log.Println("Serialized data:", tt)

	//--------------------------------------
	var decodedResult map[string]interface{}
	decoder := gob.NewDecoder(bytes.NewReader(tt))
	if err := decoder.Decode(&decodedResult); err != nil {
		log.Println("Error decoding:", err)
	}

	log.Println("Deserialized data:", decodedResult)

	//--------------------------------------

	err := s.Process.Send(
		gen.ProcessID{Name: round_mail_box, Node: s.erl_client_name},
		etf.Tuple{etf.Atom("start_round_ok"), tt, client_ids},
	)
	if err != nil {
		panic(err)
	}

	log.Printf("after sending (%#v, %#v) ! (start_round_ok)", s.erl_worker_mailbox, s.erl_client_name)
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

	geserver := FedLangProcess{
		erl_client_name:    erl_client_name,
		erl_worker_mailbox: erl_worker_mailbox,
	}
	fcmeansserver := FCMeansServer{
		FedLangProcess: &geserver,
	}
	geserver.Callable = fcmeansserver

	proc, err := node.Spawn(experiment_id, gen.ProcessOptions{}, &geserver)
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}
	geserver.Process = proc

	err = geserver.Process.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("gorlang_node_ready"), geserver.Process.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
	log.Printf("FCMeansServer terminated\n")
}
