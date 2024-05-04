#!/bin/bash

export MYIP=$(hostname -i)
export RUN=$(date '+%Y%m%d%H%M')
export FL_CLIENT_ID="-1"
export PROJECT_PATH="/app"
export ERLLIBDIR=$PROJECT_PATH/Pyrlang/py.erl
export PYTHONPATH=$PYTHONPATH:$PROJECT_PATH/src
export FL_CLIENT_LOG_FOLDER=$PROJECT_PATH/logs
export FL_DIRECTOR_PY_DIR=$PROJECT_PATH/src/python_server
export FL_DIRECTOR_CONFIG_DIR=$PROJECT_PATH/configs/server
export METRIC_FILE="$PROJECT_PATH/stats/memory_by_method_col_strategy.log"
export RUNTIME_FILE="$PROJECT_PATH/stats/runtime_by_method_col_strategy.log"

echo "CURRENT PATH: ${PROJECT_PATH}"
erlc $PROJECT_PATH/src/erlang_server/*.erl
epmd -daemon
sleep 10
erl -connect_all false -name $FL_DIRECTOR_NAME -setcookie $FL_COOKIE -noshell -s director main
