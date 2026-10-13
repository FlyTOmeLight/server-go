#!/bin/sh
set -eu
# Run from the example root after configuring the CLI endpoint.
go run ./cmd/cli module create --name=example --project-id=project1
go run ./cmd/cli module list
# Use the id returned by create for get, update and delete.
