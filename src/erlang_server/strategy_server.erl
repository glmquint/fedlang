-module(strategy_server).
-author('José Luis Corcuera Bárcena <joseluis.corcuera@phd.unipi.it>').
-author('Fabio Buchignani <f.buchignani@studenti.unipi.it>').
-export([init_strategy_server/5]).
-import(round, [start/7]).

termcon_max_rounds(PyPid, RoundNumber, TermConParams) ->
  MaxNumRounds = TermConParams,
  RoundNumber < MaxNumRounds.

termcon_custom(PyPid, RoundNumber, TermConParams) ->
  {MaxNumRounds} = TermConParams,
  if
    RoundNumber < MaxNumRounds ->
      PyPid ! {self(), 'next_round'},
      receive
        {next_round_ok, Results} -> AnotherRound = Results
      end,
      AnotherRound;
    true -> false
  end.

termcon_metric_under_threshold(PyPid, RoundNumber, TermConParams) ->
  {MaxNumRounds, Threshold } = TermConParams,
  io:format("termcon_metric_under_threshold: RoundNumber = ~p, MaxNumRounds = ~p, Threshold = ~p  ~n", [RoundNumber, MaxNumRounds, Threshold]),
  if
    RoundNumber < MaxNumRounds ->
      PyPid ! {self(), 'get_current_metric_value'},
      io:format("calling get_current_metric_value ~n", []),
      receive
        {get_current_metric_value_ok, MetricValue} -> AnotherRound = MetricValue > Threshold
      end,
      io:format("termcon_metric_under_threshold: MetricValue > Threshold (~p > ~p), AnotherRound = ~p ~n", [MetricValue, Threshold, AnotherRound]),
      AnotherRound;
    true -> false
  end.

termcon_metric_over_threshold(PyPid, RoundNumber, TermConParams) ->
  {MaxNumRounds, Threshold} = TermConParams,
  io:format("termcon_metric_over_threshold: RoundNumber = ~p, MaxNumRounds = ~p, Threshold = ~p  ~n", [RoundNumber, MaxNumRounds, Threshold]),
  if
    RoundNumber < MaxNumRounds ->
      PyPid ! {self(), 'get_current_metric_value'},
      io:format("calling get_current_metric_value ~n", []),
      receive
        {get_current_metric_value_ok, MetricValue} -> AnotherRound = MetricValue < Threshold
      end,
      io:format("termcon_metric_over_threshold: MetricValue < Threshold (~p > ~p), AnotherRound = ~p ~n", [MetricValue, Threshold, AnotherRound]),
      AnotherRound;
    true -> false
  end.

send_stats_message(StatsNodePID, MessageToBeSent) ->
    {MegaSecs, Secs, _} = now(),
    UnixTime = MegaSecs * 1000000 + Secs,
    StatsNodeMessage = lists:flatten(io_lib:format(MessageToBeSent, [UnixTime])),
    StatsNodePID ! {fl_message, StatsNodeMessage}.

create_python_client(ExperimentID, PythonModule, PyNodeName, WorkerName, WorkerMailBox) ->
    Cookie = os:getenv("FL_COOKIE"),
    PythonScriptDir = os:getenv("FL_DIRECTOR_PY_DIR"),
    io:format("python3 -u ~p/~p.py ~p ~p ~p ~p ~p ~n",[PythonScriptDir, PythonModule, PyNodeName, WorkerName, WorkerMailBox, Cookie, ExperimentID]),
    %S = io_lib:format("mprof run --output stats/fedlang_server_~p.dat python3 -u -m cProfile -o stats_~p.prof python_server_scripts/~p.py ~p ~p ~p ~p",[ExperimentID, ExperimentID, PythonModule, PyNodeName, WorkerName, WorkerMailBox, Cookie]),
    S = io_lib:format("python3 -u ~s/~s.py ~s ~s ~s ~s ~s",[PythonScriptDir, PythonModule, PyNodeName, WorkerName, WorkerMailBox, Cookie, ExperimentID]),
    spawn(fun() -> os:cmd(S) end).

init_strategy_server(DirectorPID, ExperimentID, Clients, ExperimentDataDescriptor, StatsNodePID) ->
    io:format("Starting aggregator for ~p ~n", [ExperimentID]),
    io:format("ExperimentDataDescriptor ~p ~n", [ExperimentDataDescriptor]),
    {Algorithm, CodeLanguage, ClientStrategy, ClientSelectionRatio, MinNumberClients, StopCondition, StopConditionThr, MaxNumRounds, JsonStrParams} = ExperimentDataDescriptor,
    io:format("Algorithm ~p ~n", [Algorithm]),
    PythonModule = Algorithm ++ "_server",
    io:format("Clients: ~p ~n", [Clients]),
    io:format("Algorithm, PythonModule: ~p, ~p ~n", [Algorithm, PythonModule]),
    io:format("ClientStrategy, ClientSelectionRatio: ~p, ~p ~n", [ClientStrategy, ClientSelectionRatio]),
    io:format("StopCondition, StopConditionThr: ~p, ~p ~n", [StopCondition, StopConditionThr]),
    io:format("MaxNumRounds: ~p ~n", [MaxNumRounds]),
    RoundNumber = 1,
    NodeName = os:getenv("FL_DIRECTOR_NAME"),
    NodeMailBox = lists:flatten(io_lib:format("mboxserver_~s",[list_to_atom(ExperimentID)])),
    register(list_to_atom(NodeMailBox), self()),
    %MYIP = os:getenv("MYIP"),
    PyNodeName = lists:flatten(io_lib:format("py_~s@127.0.0.1",[ExperimentID])),
    create_python_client(ExperimentID, PythonModule, PyNodeName, NodeName, NodeMailBox),
    ClientIDs = [ID || {ID,_} <- Clients],
    receive
        {pyrlang_node_ready, PyPid, PythonOSPID} ->
                io:format("-------SERVER FL pyrlang_node_ready ~p ~n", [PyPid]),
                PyPid ! {self(), 'init_server', ExperimentID, JsonStrParams, ClientIDs}
    end,
    receive
        {fl_server_ready, _, ClientConfig, CallsListBytes} ->
                io:format("------- fl_server_ready ~p ~p ~n", [ClientConfig, CallsListBytes])
    end,
    CallsList = [{binary_to_atom(SideBytes), binary_to_atom(NameBytes)} || {SideBytes, NameBytes} <- CallsListBytes],
    io:format("Calls list: ~p ~n", [CallsList]),
    io:format("Starting clients... ~n", []),
    lists:map(fun({_, PID}) -> PID ! { fl_start_worker, self(), ExperimentID, Algorithm, CodeLanguage, ClientConfig, StatsNodePID} end, Clients),
    ClientsFLPIDs = [receive { fl_worker_ready, ClientPID, FLPID } -> {ClientID, FLPID} end || {ClientID, ClientPID} <- Clients],
%     {MegaSecs2, Secs2, _} = now(),
%     AllWorkersReadyUnixTime = MegaSecs2 * 1000000 + Secs2,
%     AllWorkersReadyMessage = lists:flatten(io_lib:format("{\"timestamp\":~p,\"type\":\"all_workers_ready\"}", [AllWorkersReadyUnixTime])),
%     StatsNodePID ! {fl_message, AllWorkersReadyMessage},
    send_stats_message(StatsNodePID, "{\"timestamp\":~p,\"type\":\"strategy_server_ready\"}"),
    send_stats_message(StatsNodePID, "{\"timestamp\":~p,\"type\":\"all_workers_ready\"}"),
    io:format("All clients are ready!: ~p ~n", [ClientsFLPIDs]),
    StopConditionAtom = list_to_atom(StopCondition),
    io:format("StopConditionAtom: ~p ~n", [StopConditionAtom]),
    case StopConditionAtom of
      custom ->
        TermConParams = {MaxNumRounds},
        TermCheckFunction = fun termcon_custom/3;
      max_number_rounds  ->
        TermConParams = MaxNumRounds,
        TermCheckFunction = fun termcon_max_rounds/3;
      metric_under_threshold ->
        TermConParams = {MaxNumRounds, StopConditionThr},
        TermCheckFunction = fun termcon_metric_under_threshold/3;
      metric_over_threshold ->
        TermConParams = {MaxNumRounds, StopConditionThr},
        TermCheckFunction = fun termcon_metric_over_threshold/3
    end,
    loop(DirectorPID, NodeMailBox, ExperimentID, TermConParams, RoundNumber, ClientsFLPIDs, PyPid, TermCheckFunction, CallsList, StatsNodePID).

loop(DirectorPID, NodeMailBox, ExperimentID, TermConParams, RoundNumber, Clients, PyPid, TermCheckFunction, CallsList, StatsNodePID) ->
    {MegaSecs1, Secs1, _} = now(),
    UnixTime1 = MegaSecs1 * 1000000 + Secs1,
    StatsMsg1 = lists:flatten(io_lib:format("{\"timestamp\":~p,\"type\":\"start_round\",\"round\":~p}", [UnixTime1, RoundNumber])),
    StatsNodePID ! {fl_message, StatsMsg1},
    Results = round:start(ExperimentID, NodeMailBox, RoundNumber, PyPid, Clients, CallsList, StatsNodePID),
    io:format("TermConParams: ~p~n", [TermConParams]),
    AnotherRound = TermCheckFunction(PyPid, RoundNumber, TermConParams),
    io:format("AnotherRound: ~p ~n", [AnotherRound]),
    if AnotherRound ->
      NextRound = RoundNumber + 1,
      io:format("NextRound: ~p ~n", [NextRound]),
      {MegaSecs3, Secs3, _} = now(),
      UnixTime3 = MegaSecs3 * 1000000 + Secs3,
      StatsMsg3 = lists:flatten(io_lib:format("{\"timestamp\":~p,\"type\":\"end_round\",\"round\":~p}", [UnixTime3, RoundNumber])),
      StatsNodePID ! {fl_message, StatsMsg3},
      loop(DirectorPID, NodeMailBox, ExperimentID, TermConParams, NextRound, Clients, PyPid, TermCheckFunction, CallsList, StatsNodePID);
        true ->
            lists:map(fun({_, PID}) -> PID ! { fl_end, ExperimentID} end, Clients),
            PyPid ! {self(), 'finish'},
            io:format("StrategyServer, Experiment finished PyPid: ~p. ~n", [PyPid]),
            StatsNodePID ! {fl_end_str_run, ExperimentID, Results},
            DirectorPID ! {fl_end_str_run, ExperimentID, Results}
    end.