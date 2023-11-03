import paramiko
import time
import threading
from collections import defaultdict
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

# 连接配置
servers = [
    {'hostname': 'server1', 'hostip': '202.199.13.207', 'port': 22, 'username': 'root', 'password': 'root123456'},
    {'hostname': 'server2', 'hostip': '219.216.65.186', 'port': 22, 'username': 'root', 'password': 'root123456'},
    # ... 添加其他服务器的配置
]

# 采样配置
duration_minutes = 1
frequency_seconds = 5
total_samples = (duration_minutes * 60) // frequency_seconds

# 数据存储结构
cpu_data = defaultdict(list)
memory_data = defaultdict(list)
data_lock = threading.Lock()


# 服务器监控函数
def monitor_server(server):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(server['hostip'], server['port'], server['username'], server['password'])
    try:
        for _ in range(total_samples):
            # CPU利用率
            stdin, stdout, stderr = client.exec_command("top -bn1 | grep 'Cpu(s)' | awk '{print $2+$4}'")
            cpu_usage = float(stdout.read().decode().strip())
            # 内存利用率
            stdin, stdout, stderr = client.exec_command("free | grep Mem | awk '{print $3/$2 * 100.0}'")
            memory_usage = float(stdout.read().decode().strip())

            with data_lock:
                cpu_data[server['hostname']].append(cpu_usage)
                memory_data[server['hostname']].append(memory_usage)
            time.sleep(frequency_seconds)
    finally:
        client.close()


# 创建线程
threads = []
for server in servers:
    thread = threading.Thread(target=monitor_server, args=(server,))
    thread.start()
    threads.append(thread)

# 等待所有线程完成
for thread in threads:
    thread.join()

# 保存数据到CSV
with data_lock:
    cpu_usage_df = pd.DataFrame(cpu_data)
    memory_usage_df = pd.DataFrame(memory_data)
    cpu_usage_df.to_csv('cpu_usage.csv', index=False)
    memory_usage_df.to_csv('memory_usage.csv', index=False)


# 读取CSV数据，转置，并生成热力图
def plot_heatmap(csv_file, title, pdf_file):
    df = pd.read_csv(csv_file).T

    sorted_index = sorted(df.index, key=lambda x: int(x.replace('server', '')))
    df = df.reindex(sorted_index)

    plt.figure(figsize=(10, 8))
    cmap = sns.diverging_palette(230, 20, as_cmap=True)
    sns.heatmap(df, annot=False, cmap=cmap, square=True)
    # sns.heatmap(df, annot=False, cmap="viridis", square=True)
    plt.title(title)
    plt.ylabel('Server')
    plt.xlabel('Sampling Point')
    plt.xticks(rotation=0)
    plt.yticks(rotation=0)
    plt.tight_layout()
    plt.savefig(pdf_file, format='pdf')
    plt.close()

# def plot_heatmap(csv_file, title, pdf_file):
#     df = pd.read_csv(csv_file, index_col=0).T  # 转置DataFrame，并设置第一列为索引
#
#     # 对服务器进行排序，这里假设服务器名格式为"server1", "server2", ...
#     sorted_index = sorted(df.index, key=lambda x: int(x.replace('server', '')))
#     df = df.reindex(sorted_index)
#
#     # 创建图表，并确保每个单元是正方形
#     plt.figure(figsize=(12, 9))
#     cmap = sns.diverging_palette(230, 20, as_cmap=True)
#     sns.heatmap(df, annot=False, cmap=cmap, cbar=False, square=True)
#
#     plt.title(title, fontsize=16)
#     plt.ylabel('Server', fontsize=14)
#     plt.xlabel('Sampling Point', fontsize=14)
#     plt.xticks(fontsize=10)
#     plt.yticks(fontsize=10)
#
#     # 其他美化设置...
#
#     plt.tight_layout()  # 为了确保坐标标签和标题完整显示
#     plt.savefig(pdf_file, format='pdf')
#     plt.close()

# 设置绘图风格
sns.set_theme()

# 绘制CPU利用率热力图并保存为PDF
plot_heatmap('cpu_usage.csv', 'CPU Usage Heatmap', 'cpu_usage_heatmap.pdf')

# 绘制内存利用率热力图并保存为PDF
plot_heatmap('memory_usage.csv', 'Memory Usage Heatmap', 'memory_usage_heatmap.pdf')
