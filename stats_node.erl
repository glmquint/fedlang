% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(stats_node).
-export([run_and_monitor/3,test/0]).

run_and_monitor(DirectorMBoxParam,DirectorNameParam,StartStrRunMessage) ->
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

test() ->
    DirectorName = os:getenv("FL_DIRECTOR_NAME"),
    NumberClients = list_to_integer(os:getenv("FL_NUMBER_CLIENTS")),
    Language = list_to_atom(os:getenv("FL_LANGUAGE")),
    run_and_monitor(
      "mboxDirector",
      DirectorName,
      {
       "fcmeans", Language, 1, 1, NumberClients, "max_number_rounds", none, 10,
        "{
            \"algorithm\": \"fcmeans\",  
            \"clientSelectionStrategy\": 1,  
            \"minNumberClients\": 2,  
            \"stopCondition\": null,  
            \"stopConditionThreshold\": null,  
            \"maxNumberOfRounds\": 10,  
            \"parameters\": 
            {     
                \"numFeatures\": 16,     
                \"numClusters\": 10,     
                \"targetFeature\": 16,     
                \"lambdaFactor\": 2,     
                \"seed\": 10  
            }
        }"
      }
    ).
