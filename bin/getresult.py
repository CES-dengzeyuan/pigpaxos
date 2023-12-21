import os
import re
import csv
from glob import glob

# 定义正则表达式模式
pattern = re.compile(r'Throughput = (\d+\.\d+).*?\nmean = (\d+\.\d+).*?\nmin = (\d+\.\d+).*?\nmax = (\d+\.\d+).*?\nmedian = (\d+\.\d+).*?\np95 = (\d+\.\d+).*?\np99 = (\d+\.\d+).*?\np999 = (\d+\.\d+)', re.DOTALL)

# 打开CSV文件，准备写入数据
with open('data.csv', 'w', newline='') as csvfile:
    fieldnames = ['Throughput', 'Mean', 'Min', 'Max', 'Median', 'P95', 'P99', 'P999']
    writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
    writer.writeheader()

    # 使用glob模块匹配文件名模式
    for filename in glob('client.*.log'):
        with open(filename, 'r') as file:
            log_content = file.read()

            # 使用正则表达式匹配模式提取数据
            matches = pattern.search(log_content)

            # 如果匹配成功，提取数据并写入CSV文件
            if matches:
                throughput, mean, _min, _max, median, p95, p99, p999 = matches.groups()
                writer.writerow({'Throughput': throughput, 'Mean': mean, 'Min': _min, 'Max': _max, 'Median': median, 'P95': p95, 'P99': p99, 'P999': p999})
            else:
                print(f'No match found in the file: {filename}')
