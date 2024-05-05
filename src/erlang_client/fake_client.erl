% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(fake_client).
-author('José Luis Corcuera Bárcena <joseluis.corcuera@phd.unipi.it>').
-export([main/0]).
-export([enroll_in_director/2]).

enroll_in_director(ServerMBox, ServerName) ->
    {ServerMBox, ServerName} ! {fl_register_client, self()}.


create_fake_client(N, N, ServerMBox, ServerName) ->
    io:format("create_fake_client: ~p ~n", [N]),
    spawn_link(?MODULE, enroll_in_director, [ServerMBox, ServerName]);

create_fake_client(Start, N, ServerMBox, ServerName) ->
    io:format("create_fake_client: ~p ~n", [Start]),
    spawn_link(?MODULE, enroll_in_director, [ServerMBox, ServerName]),
    create_fake_client(Start + 1, N, ServerMBox, ServerName).

main() ->
    ServerMBox = list_to_atom(os:getenv("FL_SERVER_MBOX")),
    ServerName = list_to_atom(os:getenv("FL_SERVER_NAME")),
    io:format("ServerMBox: ~p ~n", [ServerMBox]),
    io:format("ServerName: ~p ~n", [ServerName]),
    Start = list_to_integer(os:getenv("FL_CLIENT_ID_START")),
    N = list_to_integer(os:getenv("FL_CLIENT_ID_END")),
    create_fake_client(Start, N, ServerMBox, ServerName).

