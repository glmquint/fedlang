% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(stats_node).
-export([run_and_monitor/3, 
        test0/0,
        test1/0]).

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

test0() ->
    run_and_monitor(
      "mboxDirector",
      "director@172.19.0.2",
      {
       "fcmeans", python, 1, 1, 2, "max_number_rounds", none, 10, 
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

test1() ->
    run_and_monitor(
      "mboxDirector", 
      "director@172.19.0.2",
      {
       "fcmeans", go, 1, 1, 2, "max_number_rounds", none, 10, 
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
