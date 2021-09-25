#!/bin/bash
VERSION=0.02

GOOS=darwin GOARCH=amd64 go build -trimpath -o load-gen-mac-amd64-$VERSION
GOOS=linux GOARCH=amd64 go build -trimpath -o load-gen-linux-amd64-$VERSION
GOOS=windows GOARCH=amd64 go build -trimpath -o load-gen-windows-amd64-$VERSION.exe