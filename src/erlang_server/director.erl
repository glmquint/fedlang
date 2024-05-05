% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(director).
-author('José Luis Corcuera Bárcena <joseluis.corcuera@phd.unipi.it>').
-export([main/0]).
-import(strategy_server, [init_strategy_server/5]).
-import(uuid, [to_string/1, v4/1]).

main() ->
    DirectorMBox = os:getenv("FL_DIRECTOR_MBOX"),
    PythonScriptDir = os:getenv("FL_DIRECTOR_PY_DIR"),
    ConfigFileDir = os:getenv("FL_DIRECTOR_CONFIG_DIR"),
    io:format("DirectorMBox: ~p ~n", [DirectorMBox]),
    io:format("ConfigFileDir: ~p ~n", [ConfigFileDir]),
    io:format("PythonScriptDir: ~p ~n", [PythonScriptDir]),
    register(list_to_atom(DirectorMBox), self()),
    io:format("ssl enabled, ssl_manager: ~p ~n", [whereis(ssl_manager)]),
    io:format("NodeId: ~p ~n", [node()]),
    io:format("Director ID ~p.~n", [self()]),
    Experiments = [],
    Clients = [],
    NextClientID = 0,
    process(Experiments, Clients, NextClientID).

process(Experiments, Clients, NextClientID) ->
    io:format("Waiting for messages... ~n"),
    receive
        {fl_start_str_run, ExperimentDataDescriptor, StatsNodePID} ->
                io:format("fl_start_str_run StatsNodePID ~p ~n !!!", [tuple_size(ExperimentDataDescriptor)]),
                ExperimentID = uuid:to_string(uuid:v4()),
                NewExperiments = Experiments ++ [ExperimentID],
                {MegaSecs, Secs, MicroSecs} = now(),
                UnixTime = MegaSecs * 1000000 + Secs,
                StatsMessage = lists:flatten(io_lib:format("{\"timestamp\":~p,\"type\":\"experiment_queued\"}", [UnixTime])),
                StatsNodePID ! {fl_start_str_run, StatsMessage},
                spawn_link(strategy_server, init_strategy_server, [self(), ExperimentID, Clients, ExperimentDataDescriptor, StatsNodePID]),
                process(NewExperiments, Clients, NextClientID);
        {fl_register_client, ClientPID} ->
                NewClients = Clients ++ [{NextClientID, ClientPID}],
                NewNextClientID = NextClientID + 1,
                ClientPID ! {fl_confirmation, self(), NextClientID},
                io:format("fl_start REGISTER ~p ~n !!!", [NextClientID]),
                io:format("New ClientPID registered: ~p ~n", [{NextClientID, ClientPID}]),
                process(Experiments, NewClients, NewNextClientID);
        {fl_unregister_client, ClientID, ClientPID} ->
                NewClients = lists:delete({ClientID, ClientPID}, Clients),
                io:format("ClientPID unregistered: ~p. ~n", [{ClientID, ClientPID}]),
                process(Experiments, NewClients, NextClientID);
        {fl_end_str_run, ExperimentID, Results} ->
                NewExperiments = lists:delete(ExperimentID, Experiments),
                io:format("Experiment: ~p has finished. ~n", [ExperimentID]),
                process(NewExperiments, Clients, NextClientID);
        T ->
            io:format("No handler defined for message: ~p ~n", [T]),
            process(Experiments, Clients, NextClientID)
    end.

