#!/bin/bash

set -e

BINDATA_ARGS="-o config/bindata.go -pkg config"

if [ "$1" == "dev" ]; then
	BINDATA_ARGS="-debug ${BINDATA_ARGS}"
	echo "Created util/bindata.go with file proxy"
else
	echo "Created util/bindata.go with all files cached"
fi

go-bindata $BINDATA_ARGS config.json

if [ "$1" == "build" ]; then
	GOOS=linux GOARCH=amd64 go build -o Maple Fleet main.go
	gcloud compute copy-files Maple Fleet web:~/Maple Fleet --project Maple Fleet
	rm Maple Fleet
fi

if [ "$1" == "dev" ]; then
	reflex -r '\.go$' -s -d none -- sh -c 'go install && Maple Fleet'
fi