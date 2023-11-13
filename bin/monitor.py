import paramiko
import time
import threading
from collections import defaultdict
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt
import json
import sys
import os

# Global variables for storing data
cpu_data = defaultdict(list)
memory_data = defaultdict(list)
data_lock = threading.Lock()


def monitor_server(server, total_samples, frequency_seconds):
    """Function to monitor server's CPU and memory usage."""
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(server['hostip'], server['port'], server['username'], server['password'])

    try:
        for _ in range(total_samples):
            # Command to get CPU usage
            stdin, stdout, stderr = client.exec_command("top -bn1 | grep 'Cpu(s)' | awk '{print $2+$4}'")
            cpu_usage = float(stdout.read().decode().strip())

            # Command to get memory usage
            stdin, stdout, stderr = client.exec_command("free | grep Mem | awk '{print $3/$2 * 100.0}'")
            memory_usage = float(stdout.read().decode().strip())

            with data_lock:
                cpu_data[server['hostname']].append(cpu_usage)
                memory_data[server['hostname']].append(memory_usage)

            time.sleep(frequency_seconds)

    finally:
        client.close()


def plot_heatmap(csv_file, title, pdf_file):
    """Function to plot heatmap from CSV data."""
    df = pd.read_csv(csv_file).T
    sorted_index = sorted(df.index, key=lambda x: int(x.replace('server', '')))
    df = df.reindex(sorted_index)

    plt.figure(figsize=(10, 8))
    cmap = sns.diverging_palette(230, 20, as_cmap=True)
    sns.heatmap(df, annot=False, cmap=cmap, square=True)
    plt.title(title)
    plt.ylabel('Server')
    plt.xlabel('Sampling Point')
    plt.xticks(rotation=0)
    plt.yticks(rotation=0)
    plt.tight_layout()
    plt.savefig(pdf_file, format='pdf')
    plt.close()


def main():
    """Main function to initiate server monitoring and plotting."""
    # Check for command line arguments
    if len(sys.argv) < 2:
        print("Usage: python3 monitor.py <log_directory>")
        sys.exit(1)

    log_directory = sys.argv[1]
    os.makedirs(log_directory, exist_ok=True)

    with open('config.json', 'r') as file:
        config = json.load(file)

    address_info = config['address']
    servers = [
        {
            'hostname': f'server{i + 1}',
            'hostip': address.split('//')[1].split(':')[0],
            'port': 22,
            'username': 'root',
            'password': 'Aa123456'
        }
        for i, address in enumerate(address_info.values())
    ]

    duration_minutes = 1
    frequency_seconds = 5
    total_samples = (duration_minutes * 60) // frequency_seconds

    threads = []
    for server in servers:
        thread = threading.Thread(target=monitor_server, args=(server, total_samples, frequency_seconds))
        thread.start()
        threads.append(thread)

    for thread in threads:
        thread.join()

    cpu_usage_csv = os.path.join(log_directory, 'cpu_usage.csv')
    memory_usage_csv = os.path.join(log_directory, 'memory_usage.csv')
    cpu_usage_pdf = os.path.join(log_directory, 'cpu_usage_heatmap.pdf')
    memory_usage_pdf = os.path.join(log_directory, 'memory_usage_heatmap.pdf')

    with data_lock:
        cpu_usage_df = pd.DataFrame(cpu_data)
        memory_usage_df = pd.DataFrame(memory_data)
        cpu_usage_df.to_csv(cpu_usage_csv, index=False)
        memory_usage_df.to_csv(memory_usage_csv, index=False)

    sns.set_theme()
    plot_heatmap(cpu_usage_csv, 'CPU Usage Heatmap', cpu_usage_pdf)
    plot_heatmap(memory_usage_csv, 'Memory Usage Heatmap', memory_usage_pdf)


if __name__ == "__main__":
    main()

