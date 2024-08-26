package common

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
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
	NumClients         int
}

var cached_messages []struct {
	dest_selector func(id, num_peers int) int
	etf.Tuple
}

func (s *FedLangProcess) Terminate(process *gen.ServerProcess, reason string) {
	log.Printf("[%s] Terminating process with reason %q", process.Name(), reason)
}

// HandleInfo
func (s *FedLangProcess) HandleInfo(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	log.Printf("[%s] HandleInfo: \n", process.Name()) //%#v\n", process.Name(), message)
	log.Printf("s = %#v\n", s)
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
		if len(res) == 0 || res[0].Interface() == nil {
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

func (s *FedLangProcess) Update_graph(clients etf.Term, fp FedLangProcess) {
	log.Printf("Update_graph: %#v\n", clients)
	clients_list := clients.(etf.List)
	s.clients = make(map[int]etf.Pid)
	for _, client := range clients_list {
		client := client.(etf.Tuple)
		log.Printf("Update_graph: client = %#v\n", client)
		client_id := client[0].(int)
		client_pid := client[2].(etf.Pid)
		s.clients[client_id] = client_pid
	}
	s.NumClients = len(s.clients)
	log.Printf("graph updated: clients = %#v, cached_messages = %#v\n", s.clients, cached_messages)
	for _, msg := range cached_messages {
		log.Printf("Sending cached message %#v\n", msg)
		s._peerSend(msg.dest_selector, msg.Tuple)
		// remove this message from the list
		// s.cached_messages = append(s.cached_messages[:idx], s.cached_messages[idx+1:]...)
	}
}

func (s *FedLangProcess) WorkerSend(msg etf.Tuple) {
	log.Printf("WorkerSend: %#v\n", msg)
	err := s.process.Send(gen.ProcessID{Name: s.erl_worker_mailbox, Node: s.erl_client_name}, msg)
	if err != nil {
		panic(err)
	}
}

// expects a function that takes an id and the number of peers and returns the id of the peer to send the message to
func (s *FedLangProcess) PeerSend(dest_selector func(id, num_peers int) int, function string, args ...interface{}) {
	msg := etf.Tuple{s.Own_pid, etf.Atom(function)}
	for _, arg := range args {
		msg = append(msg, arg)
	}
	s._peerSend(dest_selector, msg)
}

func (s *FedLangProcess) _peerSend(dest_selector func(id, num_peers int) int, msg etf.Tuple) {
	id_env := os.Getenv("FL_CLIENT_ID")
	id, _ := strconv.Atoi(id_env)
	if len(s.clients) == 0 {
		log.Printf("No clients found, caching message %#v\n", msg)
		cached_messages = append(cached_messages, struct {
			dest_selector func(id, num_peers int) int
			etf.Tuple
		}{dest_selector, msg})
		log.Printf("cached_messages = %#v\n", cached_messages)
		return
	}
	peer_id := dest_selector(id, len(s.clients))
	peer_pid := s.clients[peer_id]
	log.Printf("PeerSend: peer_id = %d, peer_pid = %#v for message %#v\n", peer_id, peer_pid, msg)
	err := s.process.Send(peer_pid, msg)
	if err != nil {
		log.Printf("peer_pid = %#v not found, caching message %#v\n", peer_pid, msg)
		cached_messages = append(cached_messages, struct {
			dest_selector func(id, num_peers int) int
			etf.Tuple
		}{dest_selector, msg})
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
		NumClients:         0,
	}
	cached_messages = make([]struct {
		dest_selector func(id, num_peers int) int
		etf.Tuple
	}, 0)
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
