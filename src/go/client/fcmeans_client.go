package main

import (
	"fcmeans/common"
)
type FCMeansClient struct {
	*common.FedLangProcess

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

func (f *FCMeansClient) init_client() {
	f.FedLangProcess = common.NewFedLangProcess()
}
func (f *FCMeansClient) process_client() {
}
func (f *FCMeansClient) destroy() {
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