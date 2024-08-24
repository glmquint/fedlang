#!/bin/bash

# expects FL_DIRECTOR_NAME, FL_NUMBER_CLIENTS, and FL_LANGUAGE to be set

echo "Starting stats node"

export FL_STATS_NODE_IP=$(hostname -I | awk '{print $1}')
echo "FL_DIRECTOR_NAME: $FL_DIRECTOR_NAME"
echo "FL_NUMBER_CLIENTS: $FL_NUMBER_CLIENTS"
echo "FL_LANGUAGE: $FL_LANGUAGE"
echo "FL_STATS_NODE_IP: $FL_STATS_NODE_IP"


erlc stats_node.erl && erl -name stats_node_test@$FL_STATS_NODE_IP -setcookie 'cookie_123456789' -kernel prevent_overlapping_partitions false -noshell -s stats_node test -s init stop | tee logs/stats.log
