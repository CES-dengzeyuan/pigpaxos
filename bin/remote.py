import json
import paramiko
import threading
import subprocess
import os


def read_config(file_path):
    with open(file_path, 'r') as file:
        return json.load(file)


def get_extra_args(algorithm, pg, fr):
    if algorithm == "pigpaxos":
        return f"-pg {pg} -fr {str(fr).lower()}"
    elif algorithm == "layerpaxos":
        return f"-npg {pg} -wfr {str(fr).lower()}"
    else:
        return ""


def start_service(server_info, algorithm, log_dir, extra_args):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        ssh.connect(server_info['ip'],
                    port=server_info['port'],
                    username=server_info['username'],
                    password=server_info['password'])
        cmd = f"./server -id {server_info['id']} -algorithm={algorithm} -log_dir {log_dir} {extra_args} &"
        ssh.exec_command(cmd)
    finally:
        ssh.close()
    # cmd = f"./server -id {server_info['id']} -algorithm={algorithm} -log_dir {log_dir} {extra_args} &"
    # print(cmd)
    # print(server_info)


def start_local_processes(log_dir, config_path):
    client_cmd = f"./client -id 1.1 -log_dir {log_dir} -config {config_path} &"
    subprocess.Popen(client_cmd, shell=True)


if __name__ == "__main__":
    config_path = 'config.json'
    algorithm = "pigpaxos"
    id_num = 9
    pg = 1
    fr = True
    username = "root"
    passwd = "Aa123456"

    config = read_config(config_path)
    extra_args = get_extra_args(algorithm, pg, fr)

    log_dir = f"{algorithm}-{id_num}-{pg}"
    os.makedirs(log_dir, exist_ok=True)

    threads = []
    for server_id, addr in config['address'].items():
        hostIP, port = addr.replace('tcp://', '').split(':')
        server_info = {
            'id': server_id,
            'ip': hostIP,
            'port': int(port),
            'username': username,
            'password': passwd,
        }
        thread = threading.Thread(target=start_service,
                                  args=(server_info, algorithm, log_dir, extra_args))
        thread.start()
        threads.append(thread)

    for thread in threads:
        thread.join()

    start_local_processes(log_dir, config_path)

    print("所有服务器和客户端已启动")
