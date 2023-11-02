#!/bin/bash
./kill.sh

# 定义变量
id_num=9                       # id的数量
algorithm=epaxos               # 算法名称
pg=1                           # pg的值
fr=true                        # fr的值
log_dir=$algorithm-$id_num-$pg # log_dir的值
rm -rf $log_dir
mkdir $log_dir

case $algorithm in
pigpaxos)
  extra_args="-pg $pg -fr $fr"
  ;;
layerpaxos)
  extra_args="-npg $pg -wfr $fr"
  ;;
*)
  extra_args=""
  ;;
esac

# 使用for循环执行命令
for ((i = 1; i <= id_num; i++)); do
  #  echo ./server -id 1.$i -algorithm=$algorithm -log_dir $log_dir $extra_args &
  ./server -id 1.$i -algorithm=$algorithm -log_dir $log_dir $extra_args &
done

./client -id 1.1 -log_dir $log_dir -config config.json &
