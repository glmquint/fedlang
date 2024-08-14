package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
)

type gorlangServer struct {
	gen.Server
	erl_client_name    string
	erl_worker_mailbox string
	target_feature     int
	max_number_rounds  int
	epsilon            int
}

var startFlTime = time.Time{}

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

func (s *gorlangServer) start_round(round_mail_box, experiment, round_number string) {
	log.Printf("round_mail_box = %#v, experiment = %#v, round_number = %#v\n", round_mail_box, experiment, round_number)
	if startFlTime == (time.Time{}) {
		startFlTime = time.Now()
	}
	log.Printf("start_fl_time = %#v\n", startFlTime)
	// TODO: result := _global_model_parameters_STUB
	log.Printf("start round result = %#v\n", "_global_model_parameters_STUB")
	log.Printf("before sending result to [(%#v, %#v)", s.erl_worker_mailbox, s.erl_client_name)
	// TODO: tt = cPickler.dumps(result)
	// TODO: call send from fcmeans_server to director

}
