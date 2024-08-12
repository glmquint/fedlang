package main

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/ergo-services/ergo/etf"
	"github.com/ergo-services/ergo/gen"
)

type gorlangServer struct {
	gen.Server
	erl_client_name    string
	erl_worker_mailbox string
}

func (s *gorlangServer) HandleCast(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	fmt.Printf("[%s] HandleCast: %#v\n", process.Name(), message)
	switch message {
	case etf.Atom("stop"):
		return gen.ServerStatusStopWithReason("stop they said")
	}
	return gen.ServerStatusOK
}

func (s *gorlangServer) HandleCall(process *gen.ServerProcess, from gen.ServerFrom, message etf.Term) (etf.Term, gen.ServerStatus) {
	fmt.Printf("[%s] HandleCall: %#v, From: %s\n", process.Name(), message, from.Pid)

	switch message.(type) {
	case etf.Atom:
		return "hello", gen.ServerStatusOK

	default:
		return message, gen.ServerStatusOK
	}
}

// HandleInfo
func (s *gorlangServer) HandleInfo(process *gen.ServerProcess, message etf.Term) gen.ServerStatus {
	fmt.Printf("[%s] HandleInfo: %#v\n", process.Name(), message)
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
			return gen.ServerStatusIgnore
		}
		if len(message_as_tuple) < 2 {
			return gen.ServerStatusIgnore
		}
		pid := message_as_tuple[0]
		fun_name, ok := message_as_tuple[1].(string)
		if !ok {
			return gen.ServerStatusIgnore
		}
		args := message_as_tuple[2:]
		args_slice := make([]interface{}, len(args))
		for i, v := range args {
			args_slice[i] = v
		}
		fmt.Printf("sender = %s, fun_name = %s", pid, fun_name)
		Call(fun_name, args_slice...)
	}
	return gen.ServerStatusOK
}

func (s *gorlangServer) Terminate(process *gen.ServerProcess, reason string) {
	fmt.Printf("[%s] Terminating process with reason %q", process.Name(), reason)
}

var StubStorage = map[string]interface{}{
	"funcA": init_server,
}

func Call(funcName string, params ...interface{}) (result interface{}, err error) {
	f := reflect.ValueOf(StubStorage[funcName])
	if len(params) != f.Type().NumIn() {
		err = errors.New("The number of params is out of index.")
		return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}
	var res []reflect.Value
	res = f.Call(in)
	result = res[0].Interface()
	return
}

func init_server(experiment, json_str_config string, bb []byte) {
	f, _ := os.Create("init_server_output.txt")
	defer f.Close()
	f.WriteString(fmt.Sprintf("experiment = %s, client_ids = %s, bb = %v", experiment, json_str_config, bb))
}
