#!/usr/bin/env bash
# Verify deployments by URL
# verify_url.sh [URL]

for i in $(seq 0 15)
do
  curl -vf "${1}"
  curl_status=$?
  if [[ $curl_status == 0 ]]; then
    exit 0
  else
    sleep $i
  fi
done

echo Could not verify environment is working properly
exit 1
