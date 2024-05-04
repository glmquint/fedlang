#!/bin/bash

export MYIP=$(hostname -I | awk '{print $1}')
export PROJECT_PATH="/app"
export ERLLIBDIR=$PROJECT_PATH/Pyrlang/py.erl
export PYTHONPATH=$PYTHONPATH:$PROJECT_PATH/src
export FL_CLIENT_PY_DIR=$PROJECT_PATH/src/python_client
export FL_CLIENT_LOG_FOLDER=$PROJECT_PATH/logs

echo "CURRENT PATH: ${PROJECT_PATH}"

for i in `seq $FL_CLIENT_ID_START $FL_CLIENT_ID_END`; do
	echo "Starting client $i"
	eval "export FL_CLIENT_ID=$i"
	echo "FL_CLIENT_ID: $FL_CLIENT_ID"
	eval "export FL_CLIENT_MBOX=node_client$i"
	echo "FL_CLIENT_MBOX: $FL_CLIENT_MBOX"
	eval "export FL_CLIENT_NAME=client$i@127.0.0.1"
	echo "FL_CLIENT_NAME: $FL_CLIENT_NAME"
	eval 'export METRIC_FILE="$PROJECT_PATH/stats/memory_by_method_col_${i}_${RUN}.log"'
	if [ $i = $FL_CLIENT_ID_END ];
  then
    erl -connect_all false -name $FL_CLIENT_NAME -setcookie $FL_COOKIE -noshell -s client main
  else
    erl -connect_all false -name $FL_CLIENT_NAME -setcookie $FL_COOKIE -noshell -s client main &
  fi
done
