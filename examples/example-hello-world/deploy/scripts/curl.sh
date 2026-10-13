#!/bin/sh
set -eu
endpoint="${ENDPOINT:-http://127.0.0.1:8080}"
curl --fail-with-body -H 'Content-Type: application/json' \
 -d '{"projectID":"project1","name":"example","description":"module"}' \
 "$endpoint/helloworld/internal/v1/CreateModule"
curl --fail-with-body -H 'Content-Type: application/json' \
 -d '{"pageNo":1,"pageSize":10}' "$endpoint/helloworld/internal/v1/ListModule"
