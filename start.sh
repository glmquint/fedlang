#!/bin/bash

# Start tmux session and run the director
tmux new-session -d -s fedlang "pipenv run sh run_director.sh"
sleep 1
# Split the window horizontally and run the first client
tmux split-window -h "pipenv run sh run_clients.sh 0 0"

# Split the first pane vertically and run the second client
tmux select-pane -t 0
tmux split-window -v "pipenv run sh run_clients.sh 1 1"
# Split the original pane vertically and run the Erlang node
tmux select-pane -t 2
tmux split-window -v "erlc stats_node.erl && erl -name stats_node_test@127.0.0.1 -setcookie 'cookie_123456789' -kernel prevent_overlapping_partitions false";
# Attach to the tmux session
tmux -2 attach-session -d
