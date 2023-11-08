import pandas as pd
import paramiko
from getpass import getpass
import threading

excel_path = '../ip_list.csv'
df = pd.read_csv(excel_path)
public_ips = df['公网IP'].dropna().tolist()
username = "root"
password = "Aa123456"

commands = [
    "apt update",
    "apt install -y vim golang-go git wget python3 net-tools iputils-ping",
    "pip install paramiko",
    "pip install pandas",
    "pip install seaborn",
    "git clone https://github.com/CES-dengzeyuan/pigpaxos.git"
]

def execute_commands(ip, username, password, commands):
    try:
        ssh = paramiko.SSHClient()
        ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        ssh.connect(hostname=ip, username=username, password=password)

        for command in commands:
            stdin, stdout, stderr = ssh.exec_command(command)
            print(stdout.read().decode())
            err = stderr.read().decode()
            if err:
                print(f"Error occurred while executing command on {ip}: {err}")

        ssh.close()

    except Exception as e:
        print(f"An error occurred with {ip}: {e}")

# 使用多线程来执行命令
threads = []

for ip in public_ips:
    thread = threading.Thread(target=execute_commands, args=(ip, username, password, commands))
    threads.append(thread)
    thread.start()

# 等待所有线程完成
for thread in threads:
    thread.join()

print("Commands executed on all servers.")
