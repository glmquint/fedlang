#!/bin/bash

echo "Starting clients"

export RUN=$(date '+%Y%m%d%H%M')
export PROJECT_PATH="$PWD"
export FL_SERVER_MBOX=mboxDirector
export FL_SERVER_NAME=director@127.0.0.1
export ERLLIBDIR=$PROJECT_PATH/Pyrlang/py.erl
export FL_CLIENT_LOG_FOLDER=$PROJECT_PATH/logs
export PYTHONPATH=$PYTHONPATH:$PROJECT_PATH/src
export FL_CLIENT_PY_DIR=$PROJECT_PATH/src/python_client
export FL_COOKIE=cookie_123456789

export HEART_BEAT_TIMEOUT=120

for pid in $(ps -ef | awk '/python3 -u -m cProfile -o / {print $2}'); do kill -9 $pid; done
for pid in $(ps -ef | awk '/python3 -u python_client/ {print $2}'); do kill -9 $pid; done

cd $PROJECT_PATH/src/erlang_client
erlc *.erl

for i in `seq $1 $2`; do
	echo "Starting client $i"
	eval "export FL_CLIENT_ID=$i"
	eval "export FL_CLIENT_MBOX=node_client$i"
	eval "export FL_CLIENT_NAME=client$i@127.0.0.1"
	eval 'export METRIC_FILE="$PROJECT_PATH/stats/memory_by_method_col_${i}_${RUN}.log"'
	eval "export FL_CLIENT_CONFIG_DIR=$PROJECT_PATH/configs/client$i"
  erl -connect_all -name $FL_CLIENT_NAME -setcookie $FL_COOKIE -noshell -s client main -kernel prevent_overlapping_partitions false -kernel net_ticktime 240 &
done

# This will allow you to use CTRL+C to stop all background processes
trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM
# Wait for all background processes to complete
wait
