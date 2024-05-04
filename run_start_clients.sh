#!/bin/bash

for i in `seq $1 $2`; do
	echo "start client $i"
  docker start fedlang-client_$i
done
