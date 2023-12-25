#!/bin/bash

source_file="config.json"
destination_base="config"

# 循环遍历生成目标文件名，并执行复制操作
for i in {1..25}
do
    destination_file="${destination_base}${i}.json"
    cp "$source_file" "$destination_file"
    echo "Copied $source_file to $destination_file"
done
