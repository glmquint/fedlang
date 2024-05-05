% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(stats_node).
-export([run_and_monitor/3]).

run_and_monitor(DirectorMBoxParam, DirectorNameParam, StartStrRunMessage) ->
    DirectorMBox = list_to_atom(DirectorMBoxParam),
    DirectorName = list_to_atom(DirectorNameParam),
    io:format("DirectorMBox: ~p ~n", [DirectorMBox]),
    io:format("DirectorName: ~p ~n", [DirectorName]),
    {DirectorMBox, DirectorName} ! {fl_start_str_run, StartStrRunMessage, self()},
    receive_stats().

receive_stats() ->
        io:format("Waiting for messages... ~n"),
    receive
        T ->
            io:format("Message received: ~p ~n", [T])
    end,
    receive_stats().
