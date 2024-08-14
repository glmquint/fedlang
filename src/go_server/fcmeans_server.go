package main

import (
	"log"
	"os"

	"github.com/ergo-services/ergo"
	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/ergo-services/ergo/node"
)

func main() {
	go_node_id := os.Args[1]         // go_c0ecdfb7-00f1-4270-8e46-d835bd00f153@127.0.0.1
	erl_client_name := os.Args[2]    // director@127.0.0.1
	erl_worker_mailbox := os.Args[3] // mboxserver_c0ecdfb7-00f1-4270-8e46-d835bd00f153
	erl_cookie := os.Args[4]         // cookie_123456789
	experiment_id := os.Args[5]      // c0ecdfb7-00f1-4270-8e46-d835bd00f153

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
		panic(err)
	}

	geserver := gorlangServer{
		erl_worker_mailbox: erl_worker_mailbox,
		erl_client_name:    erl_client_name,
	}

	geserver.proc, err = node.Spawn(experiment_id, gen.ProcessOptions{}, &geserver)
	if err != nil {
		panic(err)
	}

	log.Printf("gorlang_node_id = %v, erl_client_name = %v, erl_worker_mailbox = %v, erl_cookie = %v\n", go_node_id, erl_client_name, erl_worker_mailbox, erl_cookie)

	err = geserver.proc.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("gorlang_node_ready"), geserver.proc.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
}
