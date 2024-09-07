#!/bin/bash

set -e

curl -Lo aws-sam-cli.zip https://github.com/aws/aws-sam-cli/releases/latest/download/aws-sam-cli-linux-x86_64.zip
unzip aws-sam-cli.zip -d aws-sam-installation
sudo ./aws-sam-installation/install

rm -rf aws-sam-cli.zip aws-sam-installation
