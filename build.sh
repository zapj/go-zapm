#!/usr/bin/env bash

BuildDate=$(date '+%Y-%m-%d')
go build -ldflags="-X main.BuildDate=${BuildDate}" -o zapm cmd/zapm.go