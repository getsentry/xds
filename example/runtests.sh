#!/bin/bash

set -eu

SCRIPT_DIR=$(realpath "$(dirname "$0")")
XDS_DIR=$(dirname "${SCRIPT_DIR}")

function log {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')]: $1"
}

function check_kubectl_context {
  local k8s_ctx
  k8s_ctx=$(kubectl config current-context)
  echo "Test runner creates/deletes resources using kubectl"
  echo "Current kubectl context: ${k8s_ctx}"
  read -rp "Continue (y/n)? " choice
  case "$choice" in 
    y|Y ) echo;;
    * ) exit 1;;
  esac
}

function main {

  check_kubectl_context

  log "Building xds docker image"
  docker build -t xds "${XDS_DIR}"

  log "Building envoy image"
  docker build -t xdsenvoy "${SCRIPT_DIR}/envoy"


  log "Deploy to k8s"
  kubectl apply -f "${SCRIPT_DIR}/k8s"
  trap 'kubectl delete -f "${SCRIPT_DIR}/k8s"' exit

  log "Wait a bit ..."
  sleep 5

  log "Restart envoy to pick up endpoints correctly"
  kubectl scale deployment envoy --replicas 0
  kubectl scale deployment envoy --replicas 1

  log "Wait a bit ..."
  sleep 5

  local xds_port;
  local envoy_admin_port;
  local envoy_foo_port;

  xds_port=$(kubectl get service xds -o json | jq '.spec.ports[0].nodePort')
  envoy_admin_port=$(kubectl get service envoy -o json | jq '.spec.ports[] | select(.name=="admin") | .nodePort')
  envoy_foo_port=$(kubectl get service envoy -o json | jq '.spec.ports[] | select(.name=="foo") | .nodePort')

  log "Running tests"
  XDS_URL="http://localhost:${xds_port}" \
  ENVOY_ADMIN_URL="http://localhost:${envoy_admin_port}" \
  ENVOY_FOO_URL="http://localhost:${envoy_foo_port}" \
  SCRIPT_DIR="${SCRIPT_DIR}" \
    bats tests/

}

main "$@"
