#!/bin/bash

if [[ -z "$OVERLAY" ]]; then
  echo "The OVERLAY environment variable must be set"
  exit 1
fi

overlay=$OVERLAY

if [[ -z "$REGISTRY" ]]; then
  registry="docker.io"
else
  registry=$REGISTRY
fi

if [[ -z "$ORG" ]]; then
  echo "The ORG environment variable must be set"
  exit 1
fi

org=$ORG

if [[ -z "$TAG" ]]; then
  tag="latest"
else
  tag="$TAG"
fi

mkdir -p test/config/deploy_reaper_test/overlays/forks/$overlay
kustomization_file=test/config/deploy_reaper_test/overlays/forks/$overlay/kustomization.yaml

if [ ! -f $kustomization_file ]; then
  cat <<EOT >> test/config/deploy_reaper_test/overlays/forks/$overlay/kustomization.yaml
resources:
  - ../../../base

images:
  - name: controller
    newName: $registry/$org/reaper-operator
    newTag: $tag
EOT
fi
