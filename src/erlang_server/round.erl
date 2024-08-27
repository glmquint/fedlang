% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(round).
-author('José Luis Corcuera Bárcena <joseluis.corcuera@phd.unipi.it>').
-author('Fabio Buchignani <f.buchignani@studenti.unipi.it>').
-export([start/7]).

start(ExperimentID, NodeMailBox, Round, PyPid, Clients, [NextCall|CallsList], StatsNodePID) ->
    io:format("Starting round ~p for experiment ~p ~n", [Round, ExperimentID]),
    io:format("ExperimentID: ~p ~n", [ExperimentID]),
    io:format("PyPid: ~p ~n", [PyPid]),
    {RoundType, FunctionName} = NextCall,
    io:format("FunctionName: ~p ~n", [FunctionName]),
    if RoundType==standard ->
        if FunctionName=='one_step' ->
          RoundFunction = fun process_one_step_round/6;
        true ->
          RoundFunction = fun process_two_step_round/6
        end,
        RoundFunction(ExperimentID, Round, PyPid, NodeMailBox, Clients, StatsNodePID);
    true ->
        PyPid ! {self(), FunctionName, ExperimentID, Round},
        Data = undefined,
        ClientIDs = undefined,
        SelectedPIDs = [proplists:get_value(ID,Clients) || ID<-ClientIDs],
        process_custom_round(ExperimentID, Round, PyPid, SelectedPIDs, Clients, Data, CallsList, StatsNodePID)
    end.

process_one_step_round(ExperimentID, Round, PyPid, NodeMailBox, Clients, StatsNodePID) ->
  io:format("process_one_step_round ========== ~n", []),
  PyPid ! {self(), 'start_round', NodeMailBox, ExperimentID, Round},
  receive
        {start_round_ok, InitData, ClientIDs} ->
            io:format("start_round_ok ~n", [])
  end,
  NClients = length(ClientIDs),
  SelectedPIDs = [proplists:get_value(ID,Clients) || ID<-ClientIDs],
  lists:map(fun(ClientPID) -> ClientPID!{fl_next_round_step, self(), InitData, Round, 'process_client'} end, SelectedPIDs),
  AggregatedData = aggregate_worker_responses(ExperimentID, Round, NClients, []),
  %AggregatedData = process_client_sequential(InitData, Round, SelectedPIDs, []),
  PyPid ! {self(), 'process_server', NodeMailBox, ExperimentID, Round, AggregatedData},
  receive
        {process_server_ok, Results, MetricsMessage} ->
            io:format("process_server_ok ================================ ~n", [])
  end,
  StatsNodePID ! {fl_message, MetricsMessage},
  Results.

process_two_step_round(ExperimentID, Round, PyPid, NodeMailBox, Clients, StatsNodePID) ->
  io:format("process_two_step_round ========== ~n", []),
  PyPid ! {self(), 'start_round', NodeMailBox, ExperimentID, Round},
  receive
        {start_round_ok, InitData, ClientIDs} ->
            io:format("start_round_ok ~n", [])
  end,
  NClients = length(ClientIDs),
  SelectedPIDs = [proplists:get_value(ID,Clients) || ID<-ClientIDs],
  lists:map(fun(ClientPID) -> ClientPID!{fl_next_round_step, self(), InitData, Round, 'process_client'} end, SelectedPIDs),
  AggregatedProcessData = aggregate_worker_responses(ExperimentID, Round, NClients, []),
  PyPid ! {self(), 'process_server', NodeMailBox, ExperimentID, Round, AggregatedProcessData},
  receive
        {process_server_ok, ValidateParameters, ValClientIDs} ->
            io:format("process_server_ok ~n", [])
  end,
  ValNClients = length(ValClientIDs),
  ValSelectedPIDs = [proplists:get_value(ID,Clients) || ID<-ValClientIDs],
  lists:map(fun(ClientPID) -> ClientPID!{fl_next_round_step, self(), ValidateParameters, Round, 'validate_client'} end, ValSelectedPIDs),
  AggregatedValidateData = aggregate_worker_responses(ExperimentID, Round, ValNClients, []),
  PyPid ! {self(), 'validate_server', NodeMailBox, ExperimentID, Round, AggregatedValidateData},
  receive
        {validate_server_ok, Results, MetricsMessage} ->
            io:format("validate_server_ok ~n", [])
  end,
  StatsNodePID ! {fl_message, MetricsMessage},
  Results.

process_custom_round( _ , _, _, _, _, LastData, [], _) ->
   LastData;
process_custom_round(ExperimentID, Round, PyPid, SelectedPIDs, Clients, Data, [NextCall|CallsList], StatsNodePID) ->
    {Side, FunctionName} = NextCall,
    if Side==server ->
      NewData = undefined,
      NewIDs = undefined,
      NewSelectedPIDs =  [proplists:get_value(ID,Clients) || ID<-NewIDs];
    true ->
      NClients = length(SelectedPIDs),
      lists:map(fun(ClientPID) -> ClientPID!{fl_next_round_step, self(), Data, Round, FunctionName} end, SelectedPIDs),
      NewData = aggregate_worker_responses(ExperimentID, Round, NClients, []),
      NewSelectedPIDs = SelectedPIDs
    end,
    process_custom_round(ExperimentID, Round, PyPid, NewSelectedPIDs, Clients, NewData, CallsList, StatsNodePID).

aggregate_worker_responses(_ , _, 0, ClientResponses) ->
  ClientResponses;
aggregate_worker_responses(ExperimentID, Round, NClients, Results) ->
    receive
        {fl_worker_results_ack, {ClientID, DurationMS}} ->
            UpdatedNClients = NClients,
            io:format("ACK from Client ID: ~p ~n", [ClientID]),
            io:format("Client duration in MS: ~p ~n", [DurationMS]),
            io:format("Num. pending client results: ~p ~n", [UpdatedNClients]),
            aggregate_worker_responses(ExperimentID, Round, UpdatedNClients, Results);
        {fl_worker_results, {ClientInfo, DurationMS, ClientResult, NumResults}} ->
                UpdatedNClients = max(NClients - NumResults, 0),
                NewResults = Results ++ [{ClientInfo, ClientResult}],
                io:format("Client info: ~p ~n", [ClientInfo]),
                io:format("Client duration in MS: ~p ~n", [DurationMS]),
                io:format("Num. pending client results: ~p ~n", [UpdatedNClients]),
          aggregate_worker_responses(ExperimentID, Round, UpdatedNClients, NewResults);
        {fl_worker_results, {ClientInfo, DurationMS, ClientResult}} ->
        		UpdatedNClients = NClients - 1,
                NewResults = Results ++ [{ClientInfo, ClientResult}],
                io:format("Client info: ~p ~n", [ClientInfo]),
                io:format("Client duration in MS: ~p ~n", [DurationMS]),
                io:format("Num. pending client results: ~p ~n", [UpdatedNClients]),
          aggregate_worker_responses(ExperimentID, Round, UpdatedNClients, NewResults);
        {fl_step_failed} ->
        	io:format("Step failed handler: ~p ~n", [fl_step_failed]);
        T ->
            io:format("round:aggregate_client_responses - No handler defined for message: ~p ~n", [T]),
          aggregate_worker_responses(ExperimentID, Round, NClients, Results)
    end.
