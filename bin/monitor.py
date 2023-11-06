import paramiko
import time
import threading
from collections import defaultdict
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt
import json


with open('config.json', 'r') as file:
    config = json.load(file)

address_info = config['address']
servers = [
    {
        'hostname': f'server{i+1}',
        'hostip': address.split('//')[1].split(':')[0],
        'port': 22,
        'username': 'root',
        'password': 'Aa123456'
    }
    for i, address in enumerate(address_info.values())
]

# servers = [
#     {'hostname': 'server1', 'hostip': '172.24.212.230', 'port': 22, 'username': 'root', 'password': 'Aa123456'},
#     {'hostname': 'server2', 'hostip': '172.24.212.231', 'port': 22, 'username': 'root', 'password': 'Aa123456'},
#     {'hostname': 'server3', 'hostip': '172.24.212.232', 'port': 22, 'username': 'root', 'password': 'Aa123456'},
# ]

duration_minutes = 1
frequency_seconds = 5
total_samples = (duration_minutes * 60) // frequency_seconds

cpu_data = defaultdict(list)
memory_data = defaultdict(list)
data_lock = threading.Lock()


def monitor_server(server):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(server['hostip'], server['port'], server['username'], server['password'])
    try:
        for _ in range(total_samples):
            stdin, stdout, stderr = client.exec_command("top -bn1 | grep 'Cpu(s)' | awk '{print $2+$4}'")
            cpu_usage = float(stdout.read().decode().strip())
            stdin, stdout, stderr = client.exec_command("free | grep Mem | awk '{print $3/$2 * 100.0}'")
            memory_usage = float(stdout.read().decode().strip())

            with data_lock:
                cpu_data[server['hostname']].append(cpu_usage)
                memory_data[server['hostname']].append(memory_usage)
            time.sleep(frequency_seconds)
    finally:
        client.close()


threads = []
for server in servers:
    thread = threading.Thread(target=monitor_server, args=(server,))
    thread.start()
    threads.append(thread)

for thread in threads:
    thread.join()

with data_lock:
    cpu_usage_df = pd.DataFrame(cpu_data)
    memory_usage_df = pd.DataFrame(memory_data)
    cpu_usage_df.to_csv('cpu_usage.csv', index=False)
    memory_usage_df.to_csv('memory_usage.csv', index=False)


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

sns.set_theme()

plot_heatmap('cpu_usage.csv', 'CPU Usage Heatmap', 'cpu_usage_heatmap.pdf')
plot_heatmap('memory_usage.csv', 'Memory Usage Heatmap', 'memory_usage_heatmap.pdf')
