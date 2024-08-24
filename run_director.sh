#!/bin/bash

echo "Starting director"

export PROJECT_PATH="$PWD"
export FL_CLIENT_ID="-1"
#export ERLLIBDIR=$PROJECT_PATH/Pyrlang/py.erl
export FL_CLIENT_LOG_FOLDER=$PROJECT_PATH/logs
export FL_DIRECTOR_PY_DIR=$PROJECT_PATH/src/python_server
export FL_DIRECTOR_GO_DIR=$PROJECT_PATH/src/go/server
export FL_DIRECTOR_CONFIG_DIR=$PROJECT_PATH/configs/server
export FL_COOKIE=cookie_123456789
export FL_DIRECTOR_MBOX=mboxDirector
export FL_DIRECTOR_IP=$(hostname -I | awk '{print $1}')
export FL_DIRECTOR_NAME=director@$FL_DIRECTOR_IP
export PYTHONPATH=$PYTHONPATH:$PROJECT_PATH/src

export METRIC_FILE="$PROJECT_PATH/stats/memory_by_method_col_strategy.log"
export RUNTIME_FILE="$PROJECT_PATH/stats/runtime_by_method_col_strategy.log"

cd $PROJECT_PATH/src/erlang_server
rm -f *.beam
erlc *.erl
erl -connect_all -name $FL_DIRECTOR_NAME -setcookie $FL_COOKIE -noshell -s director main -kernel prevent_overlapping_partitions false -kernel net_ticktime 240

#if [ $# = 1 ] && [ $1 = 'ssl' ]; then
#  export BOOT_FILE=$PROJECT_PATH/ssl_files/ssl_jose
#  export DIRECTOR_PEM_FILE=$PROJECT_PATH/ssl_files/director_certs/director.pem
#  ERL_FLAGS="-boot $BOOT_FILE -proto_dist inet_tls
#  -ssl_dist_opt server_certfile $DIRECTOR_PEM_FILE
#  -ssl_dist_opt server_secure_renegotiate true client_secure_renegotiate true
#  -ssl_dist_opt server_verify verify_none client_verify verify_none"
#  export ERL_FLAGS
#  echo "HEREEEE"
#  erl -boot $BOOT_FILE -proto_dist inet_tls -ssl_dist_optfile $SSL_CONF_FILE -noshell -s director main $FL_COOKIE "$FL_DIRECTOR_MBOX" $FL_DIRECTOR_NAME $FL_DIRECTOR_PY_DIR $FL_DIRECTOR_CONFIG_DIR -s init stop
#fi
