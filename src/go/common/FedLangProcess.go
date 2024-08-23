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
	process            gen.Process
	Own_pid            etf.Pid
	erl_client_name    string
	erl_worker_mailbox string
	federated_actor    any
	clients            map[int]etf.Pid
	cached_messages    map[int][]etf.Term
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

		// TODO: consider inverting those checks
		f := reflect.ValueOf(s).MethodByName(funcName)
		if !f.IsValid() {
			log.Printf("Function %s is not valid. Trying to find it in the FedLangProcess\n", funcName)
			f = reflect.ValueOf(s.federated_actor).MethodByName(funcName)
			if !f.IsValid() {
				panic("The function is not valid. Must be exported (starts with a capital letter).")
			}
		}
		if len(args_slice) != f.Type().NumIn() {
			panic("The number of params is out of index. Got " + string(len(args_slice)) + " but expected " + string(f.Type().NumIn()))
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
		// s.process.Send(gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name}, result)
		s.process.Send(pid, result)
		log.Printf("result sent\n")
	}
	return gen.ServerStatusOK
}

func (s *FedLangProcess) Update_graph(clients etf.Term, fp FedLangProcess) etf.Term {
	log.Printf("Update_graph: %#v\n", clients)
	clients_list := clients.(etf.List)
	for _, client := range clients_list {
		client := client.(etf.Tuple)
		client_id := client[0].(int)
		client_pid := client[2].(etf.Pid)
		s.clients[client_id] = client_pid
		for _, msg := range s.cached_messages[client_id] {
			s._peerSend(client_id, msg)
		}
	}
	log.Printf("graph updated: clients = %#v\n", s.clients)
	return etf.Atom("graph_updated")
}

func (s *FedLangProcess) PeerSend(peer_id int, function string, args ...interface{}) {
	log.Printf("PeerSend: peer_id = %d, function = %s, args = %#v\n", peer_id, function, args)
	msg := etf.Tuple{s.Own_pid, etf.Atom(function), args}
	s._peerSend(peer_id, msg)
}

func (s *FedLangProcess) _peerSend(peer_id int, msg etf.Term) {
	peer_pid := s.clients[peer_id]
	err := s.process.Send(peer_pid, msg)
	if err != nil {
		log.Printf("peer_pid = %#v not found, caching message %#v\n", peer_pid, msg)
		s.cached_messages[peer_id] = append(s.cached_messages[peer_id], msg)
	}
	log.Printf("message to peer %d sent = %#v\n", peer_id, msg)
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
		federated_actor:    new(T),
		clients:            make(map[int]etf.Pid),
		cached_messages:    make(map[int][]etf.Term),
	}
	// 	FedLangProcess: &FedLangProcess{
	// 		erl_client_name:    erl_client_name,
	// 		erl_worker_mailbox: erl_worker_mailbox,
	// 	},

	fedlangprocess.process, err = node.Spawn(experiment_id, gen.ProcessOptions{}, &fedlangprocess)
	if err != nil {
		log.Fatalf("Error: %v", err)
		panic(err)
	}
	fedlangprocess.Own_pid = fedlangprocess.process.Info().PID

	err = fedlangprocess.process.Send(
		gen.ProcessID{Name: erl_worker_mailbox, Node: erl_client_name},
		etf.Tuple{etf.Atom("node_ready"), fedlangprocess.process.Info().PID, os.Getpid()},
	)
	if err != nil {
		panic(err)
	}

	node.Wait()
}
