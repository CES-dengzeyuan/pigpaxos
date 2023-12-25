#!/bin/bash

destination_base="config"
server_ids=(1.1 1.2 1.3 1.4 1.5 1.6 1.7 1.8 1.9)
#algorithms=(paxos epaxos pigpaxos chainpaxos)
algorithms=(pigpaxos)
client_id=1.1
log_level=info

#for i in {1..2}; do
for ((i = 1; i <= 50; i += 4)); do
    source_file="config.json"
    destination_file="${destination_base}${i}.json"
    new_concurrency=$i

    jq ".benchmark.Concurrency = $new_concurrency" "$source_file" > "$destination_file"
    echo "Created/Modified $destination_file: Set Concurrency to $new_concurrency"

    for algorithm in "${algorithms[@]}"; do
        echo "Running on config $i algorithm $algorithm"
        for id in "${server_ids[@]}"; do
            ./server -id "$id" -algorithm="$algorithm" &
        done
        ./client -id $client_id -log_level=$log_level -config $destination_file &

        sleep 65
        ./kill.sh
        rm server.*

        sleep 60
    done
done
