##!/bin/bash
#
#destination_base="config"
#
#for i in {1..25}
#do
#    file="${destination_base}${i}.json"
#    new_concurrency=$i
#
#    # 使用jq工具修改文件中的Concurrency字段
#    jq ".benchmark.Concurrency = $new_concurrency" "$file" > tmpfile && mv tmpfile "$file"
#    echo "Modified $file: Set Concurrency to $new_concurrency"
#done

#!/bin/bash

source_file="config.json"
destination_base="config"

for i in {1..25}
do
    destination_file="${destination_base}${i}.json"
    new_concurrency=$i

    # 复制源文件并使用jq工具修改Concurrency字段
    jq ".benchmark.Concurrency = $new_concurrency" "$source_file" > "$destination_file"
    echo "Created/Modified $destination_file: Set Concurrency to $new_concurrency"
done
