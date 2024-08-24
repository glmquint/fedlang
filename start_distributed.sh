#!/bin/bash

# Function to clean up tmux session and Docker containers
cleanup() {
  tmux kill-session -t fedlang_containers
  docker stop $(docker ps -a -q)
}

# Trap the EXIT signal to ensure cleanup is run when the script exits
trap cleanup EXIT

# Start a new tmux session named 'fedlang_containers'
tmux new-session -d -s fedlang_containers
tmux set-option -g mouse on

# Run the director container in the first pane
tmux send-keys "sudo docker run -v .:/app/fedlang -it --rm --network=fednet --ip=172.19.0.2 fedlang bash -c 'pipenv run sh run_director.sh'" C-m
sleep 2

for i in `seq 0 $[$1-1]`; do
  # Split the window horizontally and run the client container
  tmux split-window -h
  tmux send-keys "sudo docker run -v .:/app/fedlang -it --rm --network=fednet -e FL_CLIENT_ID=$i fedlang bash -c 'pipenv run sh start_client.sh'" C-m
done
sleep 2

# Split the original pane vertically and run the server container
tmux split-window -v
tmux send-keys "sudo docker run -v .:/app/fedlang -it --rm --network=fednet -e FL_DIRECTOR_NAME=director@172.19.0.2 -e FL_NUMBER_CLIENTS=$1 -e FL_LANGUAGE=$2 fedlang ./start_stats_node.sh" C-m

# Adjust the layout to tiled to fit all panes
tmux select-layout tiled

# Attach to the tmux session
tmux attach-session -d
