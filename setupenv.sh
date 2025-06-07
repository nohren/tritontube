#!/bin/bash

# Add loopback aliases (needs sudo)
sudo ifconfig lo0 alias 127.0.0.2
sudo ifconfig lo0 alias 127.0.0.3

# Export env vars for this session


# Start your etcd cluster or development task
echo "Running etcd setup or other commands..."
export TOKEN=token-01
export CLUSTER_STATE=new
export NAME_1=machine-1
export NAME_2=machine-2
export NAME_3=machine-3
export HOST_1=127.0.0.1
export HOST_2=127.0.0.2
export HOST_3=127.0.0.3
export CLUSTER=${NAME_1}=http://${HOST_1}:2380,${NAME_2}=http://${HOST_2}:2380,${NAME_3}=http://${HOST_3}:2380




