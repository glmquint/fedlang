#!/bin/bash

# Start tmux session and run the director
tmux new-session -d -s fedlang "pipenv run sh run_director.sh; read"
sleep 1
# Split the window horizontally and run the first client
tmux split-window -h "pipenv run sh run_clients.sh 0 0; read"

# Split the first pane vertically and run the second client
tmux select-pane -t 0
tmux split-window -v "pipenv run sh run_clients.sh 1 1; read"
# Split the original pane vertically and run the Erlang node
sleep 1
tmux select-pane -t 2
tmux split-window "erlc stats_node.erl && erl -name stats_node_test@127.0.0.1 -setcookie 'cookie_123456789' -kernel prevent_overlapping_partitions false -noshell -s stats_node test$1 -s init stop; read"; 
# Attach to the tmux session
tmux -2 attach-session -d
