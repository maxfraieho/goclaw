#!/bin/sh
cd /home/vokov/projects/goclaw
set -a
. /home/vokov/projects/goclaw/.env.local
set +a
exec /home/vokov/projects/goclaw/goclaw
