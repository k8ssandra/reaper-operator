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

if [[ -z "$TAG" ]]; then
  tag="latest"
else
  tag="$TAG"
fi

mkdir -p test/config/deploy_reaper_test/overlays/forks/$overlay

cat <<EOT >> test/config/deploy_reaper_test/overlays/forks/$overlay/kustomization.yaml
resources:
  - ../../../base

images:
  - name: controller
    newName: $registry/$overlay/reaper-operator
    newTag: $tag
EOT
