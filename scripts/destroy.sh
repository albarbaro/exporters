#!/bin/bash

export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")))

# Remove resources
echo -e "[INFO] Deleting exporter..."

oc delete -k ${WORKSPACE}/deploy/overlays/local