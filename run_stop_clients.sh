#!/bin/bash

for i in `seq $1 $2`; do
	echo "stop client $i"
  docker stop fedlang-client_$i &
done
