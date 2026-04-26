#!/bin/sh
set -eu

ROOT_DIR="/home/vokov/projects/goclaw"
ENV_FILE="$ROOT_DIR/.env.local"
BIN_PATH="$ROOT_DIR/goclaw"

cd "$ROOT_DIR"

if [ ! -f "$ENV_FILE" ]; then
    echo "start-goclaw: missing env file: $ENV_FILE" >&2
    exit 1
fi

if [ ! -x "$BIN_PATH" ]; then
    echo "start-goclaw: missing executable binary: $BIN_PATH" >&2
    exit 1
fi

set -a
. "$ENV_FILE"
set +a

exec "$BIN_PATH"
