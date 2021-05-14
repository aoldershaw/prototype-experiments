#!/bin/sh

set -e -u

function stop_docker() {
  local pid=$(cat /tmp/docker.pid)
  if [ -z "$pid" ]; then
    return 0
  fi

  # if the process has already exited, kill will error, in which case we
  # shouldn't try to wait for it
  if kill -TERM $pid; then
    wait $pid
  fi
}

stop_docker
