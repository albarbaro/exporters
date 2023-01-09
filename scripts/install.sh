#!/bin/bash

export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")))
export GITHUB_TOKEN=""
export JIRA_TOKEN=""

while [[ $# -gt 0 ]]
do
    case "$1" in
        -g|--github-token)
            GITHUB_TOKEN=$2
            ;;
        -jt|--jira-token)
            JIRA_TOKEN=$2
            ;;
        *)
            ;;
    esac
    shift  # Shift each argument out after processing them
done

if [[ "${GITHUB_TOKEN}" == "" ]]; then
  echo "[ERROR] Github Token flag is missing. Use '--github-token <value>' or '-g <value>' to allow quality dashboard to make request to github"
  exit 1
fi

# Namespace
oc create namespace dora-metrics || true

# Create required secret 
kubectl create secret generic exporters-secret -n dora-metrics --from-literal=github=$GITHUB_TOKEN 

# Install
echo -e "[INFO] Deploying exporter..."

oc apply -k ${WORKSPACE}/deploy/overlays/local

echo ""
echo "Metrics can be scraped from: http://"$(oc get route/exporter -n dora-metrics -o go-template='{{.spec.host}}{{"\n"}}')"/metrics"