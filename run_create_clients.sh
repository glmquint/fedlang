#!/bin/bash

for i in `seq $1 $2`; do
	echo "creating client $i"
  docker create --init --env FL_COOKIE=cookie_123456789 --env FL_SERVER_MBOX=mboxDirector --env FL_SERVER_NAME=director@192.168.0.2 --env FL_CLIENT_ID_START=$i --env FL_CLIENT_ID_END=$i -t -h client_$i --net fedlang_network --name fedlang-client_$i fedlang/client
done
