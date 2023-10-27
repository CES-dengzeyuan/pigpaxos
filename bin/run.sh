#!/bin/bash
# 定义一个函数，接受一个参数作为服务器的id
run_server() {
  # 使用传入的参数和固定的选项执行server命令，并在后台运行
  ./server -id $1 -algorithm=pigpaxos -pg 1 -fr true &
}

# 使用for循环，从1.1到1.9，每次增加0.1，调用run_server函数
for i in $(seq 1.1 0.1 1.9)
do
  run_server $i
done

./client -id 1.1 -config config.json &
