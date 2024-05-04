#!/bin/bash

#./certificates/generate.sh

echo "Starting server"

#rm -rf stats/*.*
rm -rf logs/*.*
export RUN=$(date '+%Y%m%d%H%M')
export PROJECT_PATH="$PWD"
export ERLLIBDIR=/home/jose/repository/UNIPI/Pyrlang/py.erl
export PYTHONPATH=$(pwd)
export FL_CLIENT_LOG_FOLDER=$PROJECT_PATH/logs

for pid in $(ps -ef | awk '/python3 -u -m cProfile -o / {print $2}'); do kill -9 $pid; done
for pid in $(ps -ef | awk '/python3 -u python_client/ {print $2}'); do kill -9 $pid; done

export FL_CLIENT_ID=-1
eval 'export METRIC_FILE="./stats/memory_by_method_aggregator_${RUN}.log"'
eval 'export RUNTIME_FILE="./stats/time_by_method_aggregator_${RUN}.log"'
sh run_director.sh &
sleep 3  # Sleep for 3s to give the server enough time to start
cd $PROJECT_PATH/src/erlang_client
for i in `seq $1 $2`; do
	echo "Starting client $i"
	eval "export FL_CLIENT_ID=$i"
	eval "export FL_CLIENT_MBOX=node_client$i"
	eval "export FL_CLIENT_NAME=client$i@127.0.0.1"
	eval 'export METRIC_FILE="./stats/memory_by_method_col_${i}_${RUN}.log"'
	export FL_COOKIE=cookie_123456789
	export FL_CLIENT_PY_DIR=$PROJECT_PATH/src/python_client
	eval "export FL_CLIENT_CONFIG_DIR=$PROJECT_PATH/configs/client$i"
	export FL_SERVER_MBOX=mboxDirector
	export FL_SERVER_NAME=director@127.0.0.1
  erl -noshell -name $FL_CLIENT_NAME -setcookie $FL_COOKIE -s client main -kernel prevent_overlapping_partitions false &
done

# This will allow you to use CTRL+C to stop all background processes
trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM
# Wait for all background processes to complete
wait
