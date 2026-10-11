#!/usr/bin/env bash
set -euo pipefail

OWNER="intentd"
REPO="inrun"
OLDER_THAN_HOURS="${1:-2}"   # optional first argument, default 2 hours

now=$(date +%s)

gh api "/repos/$OWNER/$REPO/actions/caches" --paginate --jq '.actions_caches[]' | while read -r cache; do
  id=$(echo "$cache" | jq -r '.id')
  key=$(echo "$cache" | jq -r '.key')
  created_at=$(echo "$cache" | jq -r '.created_at')
  created_ts=$(date -d "$created_at" +%s)
  age_hours=$(( (now - created_ts) / 3600 ))
  if [ "$age_hours" -ge "$OLDER_THAN_HOURS" ]; then
    echo "Deleting cache id=$id key=$key age=${age_hours}h"
    gh api -X DELETE "/repos/$OWNER/$REPO/actions/caches/$id" --silent
  fi
done