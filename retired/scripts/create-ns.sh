#!/bin/bash
#
# This script will create the specified namespace if it does not exist.

if [ $# -ne 1 ]; then
  echo "Usage: create-ns.sh <namespace>"
  exit 1
fi

namespace=$1

kubectl get ns $namespace >& /dev/null

if [ $? -eq 1 ]; then
  echo "Creating namespace $namespace"
  kubectl create ns $namespace
else
  echo "Namespace $namespace already exists"
fi
