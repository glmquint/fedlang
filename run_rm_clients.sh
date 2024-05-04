#!/bin/bash

for i in `seq $1 $2`; do
	echo "rm client $i"
  docker rm fedlang-client_$i
done
