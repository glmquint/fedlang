% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(worker).
-author('José Luis Corcuera Bárcena <joseluis.corcuera@phd.unipi.it>').
-export([init_worker/9]).

create_client(ExperimentID, ServerModule, ServerNodeName, WorkerName, WorkerMailBox, CodeLanguage, ClientID) ->
  Cookie = os:getenv("FL_COOKIE"),
  case CodeLanguage of
    python ->
      PythonScriptDir = os:getenv("FL_CLIENT_PY_DIR"),
      S = io_lib:format("python3 -u ~s/~s.py ~s ~s ~s ~s ~s ~p",[PythonScriptDir, ServerModule, ServerNodeName, WorkerName, WorkerMailBox, Cookie, ExperimentID, ClientID]);
    go ->
      GoScriptDir = os:getenv("FL_CLIENT_GO_DIR"),
      io:format(GoScriptDir),
      S = io_lib:format("~s/~s ~s ~s ~s ~s ~s ~p",[GoScriptDir, ServerModule, ServerNodeName, WorkerName, WorkerMailBox, Cookie, ExperimentID, ClientID]);
    _ -> S = "echo Unsupported language"
  end,
  io:format(S),
  spawn(fun() -> os:cmd(S) end).

init_worker(ClientPID, ClientID, ClientName, StrategyServerPID, StatsNodePID, ExperimentId, Algorithm, CodeLanguage, ClientConfig) ->
    ClientModule = Algorithm ++ "_client",
    ClientIP = os:getenv("FL_CLIENT_IP"),
    io:format("ClientIP: ~p ~n", [ClientIP]),
    io:format("Starting Worker for experiment: ~p, StrategyServerPID: ~p. ~n", [ExperimentId, StrategyServerPID]),
    io:format("ClientModule: ~p ~n", [ClientModule]),
    io:format("CLIENT ~p, mboxworker_~p ~n", [ClientID, ClientID]),
    WorkerMailBox = lists:flatten(io_lib:format("mboxworker_~s",[ExperimentId])),
    PyNodeName = lists:flatten(io_lib:format("client_~p_~s@~s",[ClientID, ExperimentId, ClientIP])),
    register(list_to_atom(WorkerMailBox), self()),
    create_client(ExperimentId, ClientModule, PyNodeName, ClientName, WorkerMailBox, CodeLanguage, ClientID),
    io:format("------- WAITING node_ready ~p ~n", [ClientID]),
    receive
        {node_ready, PyrlangNodePID, PythonOSPID} ->
                io:format("------- node_ready ~p ~n", [ClientID]),
                PyrlangNodePID ! {self(), 'init_client', ExperimentId, ClientConfig},
                step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, undefined);
        T ->
            io:format("worker_fl: No handler defined for message: ~p ~n", [T])
    end.


step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, CallerPIDParam) ->
  io:format("START:CallerPID param ~p Client ~p, ~n", [CallerPIDParam,ClientID]),
	io:format("Process FL step on client ~p. ~n", [ClientID]),
	io:format("ExperimentId: ~p ~n", [ExperimentId]),
    receive
        {fl_client_ready, _} ->
                io:format("------- fl_client_ready ~p ~n", [ClientID]),
                {MegaSecs1, Secs1, _} = now(),
                UnixTime1 = MegaSecs1 * 1000000 + Secs1,
                StatsMsg1 = lists:flatten(io_lib:format("{\"timestamp\":~p,\"type\":\"worker_ready\",\"client_id\":~p}", [UnixTime1, ClientID])),
                StatsNodePID ! {fl_message, StatsMsg1},
                StrategyServerPID ! {fl_worker_ready, ClientPID, self(), PyrlangNodePID},
                step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, undefined);
        {fl_next_round_step, RoundPID, Params, CurrentRound, FunctionName} ->
                io:format("Round ~p, ClientID ~p, step ~p, in execution ~n", [CurrentRound, ClientID, FunctionName]),
                PyrlangNodePID ! {self(), FunctionName, ExperimentId, CurrentRound, Params},
                step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, RoundPID);
        {fl_py_result_ack, MetricsMessage} ->
                DurationMS = 0,
                StatsNodePID ! {fl_message, MetricsMessage},
                CallerPIDParam ! {fl_worker_results_ack, {ClientID, DurationMS}},
                step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, CallerPIDParam);
        {fl_py_result, ReturnValue, MetricsMessage, NumResults} ->
                DurationMS = 0,
                StatsNodePID ! {fl_message, MetricsMessage},
                io:format("fl_py_result: CallerPID param ~p Client ~p, ~n", [CallerPIDParam,ClientID]),
                CallerPIDParam ! {fl_worker_results, {ClientID, DurationMS, ReturnValue, NumResults}},
                step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, CallerPIDParam);
        {fl_py_result, ReturnValue, MetricsMessage} ->
                DurationMS = 0,
                StatsNodePID ! {fl_message, MetricsMessage},
                io:format("fl_py_result: CallerPID param ~p Client ~p, ~n", [CallerPIDParam,ClientID]),
                CallerPIDParam ! {fl_worker_results, {ClientID, DurationMS, ReturnValue}},
                step_fl(PyrlangNodePID, ClientPID, StrategyServerPID, ExperimentId, ClientID, StatsNodePID, CallerPIDParam);
        {fl_end, ExperimentId} ->
                PyrlangNodePID ! {self(), 'destroy'},
                io:format("FLProcess, Experiment finished: ~p, Pylang Node: ~p ~n", [ExperimentId, PyrlangNodePID]);
        T ->
            io:format("worker_fl:step_fl - No handler defined for message: ~p ~n", [T])
    end.
