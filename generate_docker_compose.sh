#!/bin/bash

# Number of clients to add
LANGUAGE=$1
NUM_CLIENTS=$2

# Base docker-compose content
cat <<EOL > docker-compose.yaml
services:
  director:
    image: fedlang
    container_name: director
    stop_grace_period: 1s
    networks:
      fednet:
        ipv4_address: 172.19.0.2
    volumes:
      - .:/app/fedlang
    command: >
      /bin/bash -c "pipenv run sh run_director.sh; read"

  stats_node:
    image: fedlang
    container_name: stats_node
    stop_grace_period: 1s
    networks:
      fednet:
        ipv4_address: 172.19.0.3
    volumes:
      - .:/app/fedlang
    environment:
        FL_DIRECTOR_NAME: director@172.19.0.2
        FL_NUMBER_CLIENTS: $NUM_CLIENTS
        FL_LANGUAGE: $LANGUAGE
    command: >
      /bin/bash -c "erlc stats_node.erl && erl -name stats_node_test@172.19.0.3 -setcookie 'cookie_123456789' -kernel prevent_overlapping_partitions false -noshell -s stats_node test -s init stop | tee logs/stats.log; read"

EOL

# Add clients dynamically
for i in $(seq 0 $((NUM_CLIENTS-1))); do
  cat <<EOL >> docker-compose.yaml
  client$i:
    image: fedlang
    container_name: client$i
    stop_grace_period: 1s
    networks:
      fednet:
        ipv4_address: 172.19.0.$((i+4))
    volumes:
      - .:/app/fedlang
    command: >
      /bin/bash -c "pipenv run sh run_clients.sh $i $i; read"

EOL
done

# Add static services
cat <<EOL >> docker-compose.yaml
networks:
  fednet:
    name: fednet
    external: true
EOL
