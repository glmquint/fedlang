#!/bin/bash

# expects FL_CLIENT_ID and FL_DIRECTOR_IP to be set

echo "Starting client"
echo "FL_CLIENT_ID: $FL_CLIENT_ID"

export RUN=$(date '+%Y%m%d%H%M')
export PROJECT_PATH="$PWD"
export FL_SERVER_MBOX=mboxDirector
export FL_SERVER_NAME=director@$FL_DIRECTOR_IP
export ERLLIBDIR=$PROJECT_PATH/Pyrlang/py.erl
export FL_CLIENT_LOG_FOLDER=$PROJECT_PATH/logs
export PYTHONPATH=$PYTHONPATH:$PROJECT_PATH/src
export FL_CLIENT_GO_DIR=$PROJECT_PATH/src/go/client
export FL_CLIENT_PY_DIR=$PROJECT_PATH/src/python_client
export FL_COOKIE=cookie_123456789
export FL_CLIENT_IP=$(hostname -I | awk '{print $1}')
export HEART_BEAT_TIMEOUT=120

cd $PROJECT_PATH/src/erlang_client
erlc *.erl
echo "Starting client $FL_CLIENT_ID"
eval "export FL_CLIENT_MBOX=node_client$FL_CLIENT_ID"
eval "export FL_CLIENT_NAME=client$FL_CLIENT_ID@$FL_CLIENT_IP"
eval 'export METRIC_FILE="$PROJECT_PATH/stats/memory_by_method_col_${FL_CLIENT_ID}_${RUN}.log"'
eval "export FL_CLIENT_CONFIG_DIR=$PROJECT_PATH/configs/client$FL_CLIENT_ID"
erl -connect_all -name $FL_CLIENT_NAME -setcookie $FL_COOKIE -noshell -s client main -kernel prevent_overlapping_partitions false -kernel net_ticktime 240
