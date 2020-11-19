#!/usr/bin/env sh

set -e

echo $SSOSYNC_GOOGLE_CREDENTIALS_SECRET | base64 -d - > $SSOSYNC_GOOGLE_CREDENTIALS

echo "ENVIRONMENT_NAME: ${ENV}"
COMMAND=${1:-"init"}

case "$COMMAND" in 
  *)
    while true; do
      echo "Initializing sync process"
      ./ssosync
      echo "sync already finished"

      echo "Waiting $COOLDOWN_TIME for next execution"
      sleep $COOLDOWN_TIME
    done
    ;;
esac
