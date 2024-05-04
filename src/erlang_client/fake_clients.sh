#!/bin/bash

export MYIP=$(hostname -I | awk '{print $1}')
export PROJECT_PATH="$PWD"
#export PROJECT_PATH="/app"
export NUM_CLIENTS=1

echo "CURRENT PATH: ${PROJECT_PATH}"

echo "Starting fake_client $NUM_CLIENTS"
eval "export FL_CLIENT_NAME=fake_clients_$NUM_CLIENTS@127.0.0.1"
echo "FL_CLIENT_NAME: $FL_CLIENT_NAME"


erl -connect_all false -name $FL_CLIENT_NAME -setcookie $FL_COOKIE -noshell -s fake_client main
