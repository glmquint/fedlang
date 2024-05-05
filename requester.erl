% Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
% SPDX-License-Identifier: Apache-2.0
-module(requester).
-export([run_experiment/4]).


run_experiment(DirectorMBoxParam, DirectorNameParam, Experiment, Times) ->
    DirectorMBox = list_to_atom(DirectorMBoxParam),
    DirectorName = list_to_atom(DirectorNameParam),
    run_experiment_inner(DirectorMBox, DirectorName, Experiment, Times).

run_experiment_inner(DirectorMBox, DirectorName, Experiment, 1) ->
    {DirectorMBox, DirectorName}!{fl_start, Experiment, Experiment};
run_experiment_inner(DirectorMBox, DirectorName, Experiment, Times) ->
    {DirectorMBox, DirectorName}!{fl_start, Experiment, Experiment},
    run_experiment_inner(DirectorMBox, DirectorName, Experiment, Times - 1).