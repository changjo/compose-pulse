#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

NAME="${APP_CONTAINER_NAME:-composepulse-local}"
IMAGE="${APP_IMAGE_NAME:-composepulse:dev}"
PORT="${LOCAL_PORT:-18087}"
DATA_DIR="$ROOT_DIR/.local/dev/data"
CONTAINERS_DIR="$ROOT_DIR/.local/dev/containers"
CONTAINER_ROOT_IN_APP="/containers"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-change-me-local-admin-password}"
DIUN_WEBHOOK_SECRET="${DIUN_WEBHOOK_SECRET:-change-me-local-diun-secret}"
MOUNT_DOCKER_SOCK="${MOUNT_DOCKER_SOCK:-1}"
LOCAL_PLATFORM="${LOCAL_PLATFORM:-}"
SKIP_BUILD="${SKIP_BUILD:-0}"

usage() {
  cat <<USAGE
Usage: $0 <start|start-hot|stop|restart|restart-hot|logs|clean>

Commands:
  start       Build and run local UI preview container
  start-hot   Build and run with hot-reload web assets (no restart for web/* edits)
  stop        Stop and remove local UI preview container
  restart     Restart local UI preview container
  restart-hot Restart hot-reload mode container
  logs        Tail local UI preview logs
  clean       Stop container and remove local preview data (.local/dev)

Environment overrides:
  LOCAL_PORT=18087
  APP_CONTAINER_NAME=composepulse-local
  APP_IMAGE_NAME=composepulse:dev
  ADMIN_USERNAME=admin
  ADMIN_PASSWORD=change-me-local-admin-password
  DIUN_WEBHOOK_SECRET=change-me-local-diun-secret
  MOUNT_DOCKER_SOCK=1
  LOCAL_PLATFORM=linux/arm64
  SKIP_BUILD=1
USAGE
}

ensure_sample_compose() {
  sample_dir="$CONTAINERS_DIR/sample-app"
  mkdir -p "$sample_dir"
  sample_compose="$sample_dir/docker-compose.yml"
  if [ ! -f "$sample_compose" ]; then
    cat >"$sample_compose" <<'YAML'
services:
  sample-app:
    image: nginx:1.27-alpine
YAML
  fi
}

build_image_if_needed() {
  if [ "$SKIP_BUILD" = "1" ] && docker image inspect "$IMAGE" >/dev/null 2>&1; then
    return
  fi
  if [ -n "$LOCAL_PLATFORM" ]; then
    docker build --platform "$LOCAL_PLATFORM" -t "$IMAGE" "$ROOT_DIR"
  else
    docker build -t "$IMAGE" "$ROOT_DIR"
  fi
}

run_start() {
  hot_reload_mode="${1:-0}"
  mkdir -p "$DATA_DIR" "$CONTAINERS_DIR"
  ensure_sample_compose

  build_image_if_needed

  if docker ps -a --format '{{.Names}}' | grep -qx "$NAME"; then
    docker rm -f "$NAME" >/dev/null
  fi

  docker_args="
    -d
    --name $NAME
    -p $PORT:8087
    -e PORT=8087
    -e DB_PATH=/data/app.db
    -e CONTAINER_ROOT=$CONTAINER_ROOT_IN_APP
    -e ADMIN_USERNAME=$ADMIN_USERNAME
    -e ADMIN_PASSWORD=$ADMIN_PASSWORD
    -e DIUN_WEBHOOK_SECRET=$DIUN_WEBHOOK_SECRET
    -v $DATA_DIR:/data
    -v $CONTAINERS_DIR:$CONTAINER_ROOT_IN_APP:ro
  "

  if [ "$hot_reload_mode" = "1" ]; then
    docker_args="$docker_args -e WEB_DIR=/app/web-live -v $ROOT_DIR/web:/app/web-live:ro"
  fi

  if [ -n "$LOCAL_PLATFORM" ]; then
    docker_args="$docker_args --platform $LOCAL_PLATFORM"
  fi

  if [ "$MOUNT_DOCKER_SOCK" = "1" ] && [ -S /var/run/docker.sock ]; then
    docker_args="$docker_args -v /var/run/docker.sock:/var/run/docker.sock"
  fi

  # shellcheck disable=SC2086
  docker run $docker_args "$IMAGE" >/dev/null

  cat <<INFO
Local UI preview is running.
URL: http://localhost:$PORT
Login: $ADMIN_USERNAME / $ADMIN_PASSWORD
DIUN Secret: $DIUN_WEBHOOK_SECRET

Tip:
  - If you only want UI/API preview, current defaults are enough.
  - If you want update/prune to execute on local Docker, keep MOUNT_DOCKER_SOCK=1.
  - Use 'start-hot' for web/* hot reload without restart.
INFO
}

run_stop() {
  if docker ps -a --format '{{.Names}}' | grep -qx "$NAME"; then
    docker rm -f "$NAME" >/dev/null
    echo "stopped: $NAME"
  else
    echo "container not found: $NAME"
  fi
}

run_logs() {
  docker logs -f "$NAME"
}

run_clean() {
  run_stop || true
  rm -rf "$ROOT_DIR/.local/dev"
  echo "removed: $ROOT_DIR/.local/dev"
}

cmd="${1:-}"
case "$cmd" in
  start)
    run_start 0
    ;;
  start-hot)
    run_start 1
    ;;
  stop)
    run_stop
    ;;
  restart)
    run_stop || true
    run_start 0
    ;;
  restart-hot)
    run_stop || true
    run_start 1
    ;;
  logs)
    run_logs
    ;;
  clean)
    run_clean
    ;;
  *)
    usage
    exit 1
    ;;
esac
