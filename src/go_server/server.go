package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"math/rand"
	"time"

	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/nlpodyssey/gopickle/pickle"
)

type gorlangServer struct {
	gen.Server
	erl_client_name    string
	erl_worker_mailbox string
	target_feature     int
	max_number_rounds  int
	epsilon            int
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
		"process_server": process_server,
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
	parameters := experiment_config["parameters"].(map[string]int)
	log.Printf("paramters->numFeatures = %#v\n", parameters["numFeatures"])
	n_features := parameters["numFeatures"]
	seed := parameters["seed"]
	n_clusters := parameters["numClusters"]
	s.target_feature = parameters["targetFeature"]
	s.max_number_rounds = experiment_config["maxNumberOfRounds"].(int) // WARN: max_number_rounds should be in parameters??
	lambda_factor := parameters["lambdaFactor"]
	stop_condition_threshold := experiment_config["stopConditionThreshold"]
	log.Printf("n_features = %#v, seed = %#v, n_clusters = %#v, target_feature = %#v, max_number_rounds = %#v, lambda_factor = %#v, stop_condition_threshold = %#v\n", n_features, seed, n_clusters, s.target_feature, s.max_number_rounds, lambda_factor, stop_condition_threshold)
	if stop_condition_threshold != nil {
		s.epsilon = stop_condition_threshold.(int)
	} else {
		s.epsilon = -1 // HACK: check logic behind stop_condition using epsilon
		log.Println("stop_condition_threshold is nil")
	}
	log.Printf("n_clusters = %#v, epsilon = %#v, max_number_rounds = %#v, n_features = %#v", n_clusters, s.epsilon, s.max_number_rounds, n_features)
}
func (s *gorlangServer) process_server(round_mail_box string, experiment string, config_file string, client_responses []interface{}){
	log.Printf("Starting process_server ...")
	log.Printf("self = %#v, round_mail_box = %#v, experiment = %#v, config_file = %#v, client_responses = %#v\n",  round_mail_box, experiment, config_file, client_responses)
	
	rand.Seed(time.Now().UnixNano())
	const low = 2
	const high = 4
	time_to_sleep := low + rand.Float64()*(high-low)
	time.Sleep(time.Duration(time_to_sleep) * time.Second)

	log.Printf("start process_server, experiment = %s, round_mail_box = %s, len(client_responses) = %d\n", experiment, round_mail_box, len(client_responses))

	var data [][]interface{}
	for _, clientResponse := range client_responses {
		data_resp, _ := pickle.Loads(clientResponse[1].([]byte))

		responseData := []interface{}{clientResponse[0], data_resp}
		data = append(data, responseData)
	}

	for i, d := range data {
		log.Printf("i = %d, data[0] = %v", i, d[0])
	}
	
	var cl_resp [][] interface{}
	for cr:= range data {
		cl_resp = append(cl_resp, cr[1])
	}

	s.current_round += 1
	num_clients := len(client_responses)

	uList := make([]float64, s.NumClusters)
	wsList := make([][]float64, s.NumClusters)
	for i := range wsList {
		wsList[i] = make([]float64, s.NumFeatures)
	}

	for client_idx := range(num_clients){
		response := client_responses[client_idx]
		for i := range(s.numClusters){
			client_u := response[0][i]
			if _, ok := interface{}(response[1][i]).(*mat.Dense); ok {
				client_ws := response[1][i]
			} else {
				client_ws := mat.NewDense(response[1][i]) // TODO: check if is numpy array (numpy equivalent in Golang)
			}
			client_ws := response[1][i] // TODO: check if is numpy array (numpy equivalent in Golang)
			uList[i] = uList[i] + client_u
			wsList[i] = wsList[i] + client_ws
	}
	var newClusterCenters [][]float64
	prevClusterCenters := s.ClusterCenters[len(s.ClusterCenters)-1]

	for i := 0; i < s.NumClusters; i++ {
		u := uList[i]
		ws := wsList[i]
		var center []float64
		if u == 0 {
			center = prevClusterCenters[i]
		} else {
			center = ws / float64(u)
		}
		newClusterCenters = append(newClusterCenters, center)
	}
	s.Experiment.SetGlobalModelParameters(newClusterCenters)
	s.ClusterCenters = append(s.ClusterCenters, newClusterCenters)

	data, clientIDs := s.Experiment.GetStepData(newClusterCenters)

	centersR := mat.NewDense(len(newClusterCenters), len(newClusterCenters[0]), flatten(newClusterCenters))
	centersR1 := mat.NewDense(len(prevClusterCenters), len(prevClusterCenters[0]), flatten(prevClusterCenters))
	diffMatrix := mat.NewDense(centersR.RawMatrix().Rows, centersR.RawMatrix().Cols, nil)
	diffMatrix.Sub(centersR, centersR1)
	v, _ := mem.VirtualMemory()
	cpuPercentage, _ := cpu.Percent(0, false)

	metricsMessage := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"round":     s.CurrentRound,
		"hostMetrics": map[string]float64{
			"cpuUsagePercentage":    cpuPercentage[0],
			"memoryUsagePercentage": v.UsedPercent,
		},
		"modelMetrics": map[string]float64{
			"FRO": totDiffSum,
		},
	}
	metricsMessageBytes, _ := json.Marshal(metricsMessage)

	// TODO: Missing send metrics to the client

}
}
