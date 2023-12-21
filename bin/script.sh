#!/bin/bash
# This script runs the server and client commands with different algorithms
# It waits for 5 seconds before killing the processes and switching the algorithm
# It waits for 2 seconds before restarting the processes with the new algorithm

server_ids=(1.1 1.2 1.3 1.4 1.5 1.6 1.7 1.8 1.9)

#algorithms=(paxos epaxos pigpaxos chainpaxos)
algorithms=(pigpaxos)

client_id=1.1
log_level=info
config=config.json

# shellcheck disable=SC2068
for algorithm in ${algorithms[@]}; do
  for id in ${server_ids[@]}; do
    ./server -id "$id" -algorithm="$algorithm" &
  done
  ./client -id $client_id -log_level=$log_level -config $config &

  sleep 5
  ./kill.sh
  sleep 2
done