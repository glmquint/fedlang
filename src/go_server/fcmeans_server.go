package main

import (
	"fmt"
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

	node, err := ergo.StartNode(go_node_id, erl_cookie, node.Options{})
	if err != nil {
		panic(err)
	}

	proc, err := node.Spawn(experiment_id, gen.ProcessOptions{}, &gorlangServer{
		erl_worker_mailbox: erl_worker_mailbox,
		erl_client_name:    erl_client_name,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("Start erlang node with the command below:")
	fmt.Printf("    $ erl -name %s -setcookie %s\n\n", "erl-"+node.Name(), erl_cookie)

	err = proc.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("gorlang_node_ready"), proc.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
}
