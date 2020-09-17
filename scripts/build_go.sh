#!/usr/bin/env bash

export GOOS=linux
export GOARCH=amd64

go test ./...

go build -ldflags="-s -w" -o bin/main cmd/lambda/main.go
zip --junk-paths bin/main.zip bin/main
aws lambda update-function-code --function-name collaborative-book --zip-file fileb://bin/main.zip
