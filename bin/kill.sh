#!/bin/bash
# This script kills all processes with the keyword "algorithm=pigpaxos"
ps -ef | grep "algorithm=" | awk '{print $2}' | xargs kill -9
