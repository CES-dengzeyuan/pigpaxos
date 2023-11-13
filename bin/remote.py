import json
import math

import paramiko
import threading
import subprocess
import os


def read_config(file_path):
    with open(file_path, 'r') as file:
        return json.load(file)


def get_extra_args(algorithm, pg, nodes):
    if algorithm == "pigpaxos":
        return f"-algorithm={algorithm} -pg {pg} "
    elif algorithm == "layerpaxos":
        return f"-algorithm=pigpaxos -pg {pg} -rgslack {nodes} "
    else:
        return ""


def start_service(server_info, log_dir, extra_args):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        ssh.connect(server_info['ip'],
                    port=server_info['port'],
                    username=server_info['username'],
                    password=server_info['password'])
        cmd = f"cd /root/pigpaxos/bin; mkdir {log_dir}"
        ssh.exec_command(cmd)
        cmd = f"cd /root/pigpaxos/bin; ./server -id {server_info['id']} -log_dir {log_dir} -log_level=info {extra_args}&"
        ssh.exec_command(cmd)
    finally:
        ssh.close()


def execute_remote_command(hostname, port, username, password, command):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(hostname, port, username, password)
    ssh.exec_command(command)
    ssh.close()


def kill_remote_processes(slpconf):
    kill_command = "pkill -f 'algorithm='"
    for server_id, addr in slpconf['config']:
        hostIP, port = addr.replace('tcp://', '').split(':')
        execute_remote_command(
            hostIP,
            22,
            slpconf['username'],
            slpconf['password'],
            kill_command
        )


def start_local_processes(log_dir, config_path, slpconf):
    monitor_cmd = f"python3 monitor.py {log_dir} &"
    print(monitor_cmd)
    subprocess.Popen(monitor_cmd, shell=True)

    client_cmd = f"./client -id 1.1 -log_dir {log_dir} -log_level=info -config {config_path}"
    client_process = subprocess.Popen(client_cmd, shell=True)
    client_process.wait()

    kill_remote_processes(slpconf)


if __name__ == "__main__":
    config_path = 'config.json'
    algorithms = ["paxos", "epaxos", "pigpaxos", "layerpaxos"]
    username = "root"
    passwd = "Aa123456"

    config = read_config(config_path)
    IPLen = len(config['address'].items())
    pg = int(math.sqrt(IPLen))
    nodes = int(IPLen / (pg * 2))

    for algorithm in algorithms:
        if algorithm == "paxos":
            extra_args = f"-algorithm={algorithm} "
        elif algorithm == "epaxos":
            extra_args = f"-algorithm={algorithm} "
        else:
            extra_args = get_extra_args(algorithm, pg, nodes)

        log_dir = f"{algorithm}-{IPLen}"
        os.makedirs(log_dir, exist_ok=True)

        threads = []
        for server_id, addr in config['address'].items():
            hostIP, port = addr.replace('tcp://', '').split(':')
            server_info = {
                'id': server_id,
                'ip': hostIP,
                'port': 22,
                'username': username,
                'password': passwd,
            }
            thread = threading.Thread(target=start_service,
                                      args=(server_info, log_dir, extra_args))
            thread.start()
            threads.append(thread)

        for thread in threads:
            thread.join()

        slpconf = {
            'config': config['address'].items(),
            'username': username,
            'password': passwd,
        }
        start_local_processes(log_dir, config_path, slpconf)

        print("所有服务器和客户端已启动")
