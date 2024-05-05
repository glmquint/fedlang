% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(client).
-author('José Luis Corcuera Bárcena <joseluis.corcuera@phd.unipi.it>').
-export([main/0]).
-import(worker, [init_worker/9]).

main() ->
    Cookie = os:getenv("FL_COOKIE"),
    ClientMBox = os:getenv("FL_CLIENT_MBOX"),
    ClientName = os:getenv("FL_CLIENT_NAME"),
    ServerMBox = os:getenv("FL_SERVER_MBOX"),
    ServerName = os:getenv("FL_SERVER_NAME"),
    io:format("Cookie: ~p ~n", [Cookie]),
    io:format("ClientMBox: ~p ~n", [ClientMBox]),
    io:format("ClientName: ~p ~n", [ClientName]),
    io:format("ServerMBox: ~p ~n", [ServerMBox]),
    io:format("ServerName: ~p ~n", [ServerName]),
    io:format("ssl enabled, ssl_manager: ~p ~n", [whereis(ssl_manager)]),
    register(list_to_atom(ClientMBox), self()),
    io:format("NodeId: ~p ~n", [node()]),
    init(ServerMBox, ServerName, ClientName, ClientMBox).

init(ServerMBox, ServerName, ClientName, ClientMBox) ->
    io:format("Client ID ~p.~n", [self()]),
    % (1) Register client in server node
    io:format("Sending message to ~p, ~p.~n", [list_to_atom(ServerMBox), list_to_atom(ServerName)]),
    {list_to_atom(ServerMBox), list_to_atom(ServerName)} ! {fl_register_client, self()},
    receive
      {fl_confirmation, DirectorPID, ClientID} ->
            io:format("Client ID ~p registered in the Director.~n", [ClientID])
    end,
    % (2) Wait for incoming messages
    loop(DirectorPID, ClientID, ClientName, ClientMBox).

loop(DirectorPID, ClientID, ClientName, ClientMBox) ->
    receive
        {fl_register} ->
            DirectorPID ! {fl_register_client, self()};
        {fl_start_worker, StrategyServerPID, ExperimentId, Algorithm, CodeLanguage, ClientConfig, StatsNodePID} ->
                spawn_link(worker, init_worker, [self(), ClientID, ClientName, StrategyServerPID, StatsNodePID, ExperimentId, Algorithm, CodeLanguage, ClientConfig]);
        {stop, StopData} ->
                io:format("client:loop - stop message was received from Director ~p ~n", [StopData]);
        T ->
            io:format("client:loop - No handler defined for message: ~p ~n", [T])
    end,
    loop(DirectorPID, ClientID, ClientName, ClientMBox).
