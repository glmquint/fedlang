package common

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/ergo-services/ergo"
	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
	"github.com/ergo-services/ergo/node"
)

type FedLangProcess struct {
	gen.Server
	gen.Process
	erl_client_name    string
	erl_worker_mailbox string
	Callable
}

func (s *FedLangProcess) Terminate(process *gen.ServerProcess, reason string) {
	log.Printf("[%s] Terminating process with reason %q", process.Name(), reason)
}

// HandleInfo
func (s *FedLangProcess) HandleInfo(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	log.Printf("[%s] HandleInfo: \n", process.Name()) //%#v\n", process.Name(), message)
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
			panic("error: cannot cast message to Tuple")
		}
		if len(message_as_tuple) < 2 {
			panic("error: message_as_tuple is < 2")
		}
		pid := message_as_tuple[0]
		fun_name := fmt.Sprintf("%v", message_as_tuple[1])
		if !ok {
			panic("error: cannot cast message_as_tuple to string")
		}
		args := message_as_tuple[2:]
		args_slice := make([]interface{}, len(args)+1)
		for i, v := range args {
			args_slice[i] = v
		}
		args_slice[len(args)] = *s
		log.Printf("sender = %#v, fun_name = %#v\n", pid, fun_name)
		// result, err := s.Callable.Call(fun_name, args_slice...)

		funcName := strings.ToUpper(string(fun_name[0])) + fun_name[1:]
		log.Printf("funcname = %s\n", funcName) // = %v\n", funcName, params)

		f := reflect.ValueOf(s.Callable).MethodByName(funcName)
		if !f.IsValid() {
			panic("The function is not valid. Must be exported (starts with a capital letter).")
		}
		if len(args_slice) != f.Type().NumIn() {
			panic("The number of params is out of index.")
		}
		in := make([]reflect.Value, len(args_slice))
		for k, param := range args_slice {
			in[k] = reflect.ValueOf(param)
		}
		var res []reflect.Value
		log.Printf("Calling %#v with in_args %#v\n", f, in)
		res = f.Call(in)
		if len(res) == 0 {
			log.Println("The function does not return any value.")
			return gen.ServerStatusOK
		}
		result := res[0].Interface()
		log.Printf("result = %#v\n", result)
		s.Process.Send(gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name}, result)
		log.Printf("result sent\n")
	}
	return gen.ServerStatusOK
}

func StartProcess[T any](go_node_id, erl_cookie, erl_client_name, erl_worker_mailbox, experiment_id string) {
	node, err := ergo.StartNode(go_node_id, erl_cookie, node.Options{})
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}

	fedlangprocess := FedLangProcess{
		erl_client_name:    erl_client_name,
		erl_worker_mailbox: erl_worker_mailbox,
		Callable:           new(T),
	}
	// 	FedLangProcess: &FedLangProcess{
	// 		erl_client_name:    erl_client_name,
	// 		erl_worker_mailbox: erl_worker_mailbox,
	// 	},

	fedlangprocess.Process, err = node.Spawn(experiment_id, gen.ProcessOptions{}, &fedlangprocess)
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}

	err = fedlangprocess.Process.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("node_ready"), fedlangprocess.Process.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
}
