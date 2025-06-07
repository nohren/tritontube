#!/bin/bash

# Remove aliases (cleanup)
sudo ifconfig lo0 -alias 127.0.0.2
sudo ifconfig lo0 -alias 127.0.0.3