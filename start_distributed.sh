#!/bin/bash

# Function to clean up tmux session and Docker containers
cleanup() {
  tmux kill-session -t fedlang_containers
  docker compose down
}

# Trap the EXIT signal to ensure cleanup is run when the script exits
trap cleanup EXIT

# Start a new tmux session named 'fedlang_containers'
tmux new-session -d -s fedlang_containers

# Run the director container in the first pane
tmux send-keys -t fedlang_containers "docker compose up director; sleep 1" C-m

# Split the window horizontally and run the client0 container
tmux split-window -h
tmux send-keys "docker compose up client0" C-m

# Split the first pane vertically and run the client1 container
tmux select-pane -t 0
tmux split-window -v
tmux send-keys "docker compose up client1; sleep 1" C-m

# Split the original pane vertically and run the server container
tmux select-pane -t 2
tmux split-window -v
tmux send-keys "docker compose up server" C-m

# Adjust the layout to tiled to fit all panes
tmux select-layout tiled

# Attach to the tmux session
tmux -2 attach-session -d