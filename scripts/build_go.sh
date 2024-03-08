#!/usr/bin/env bash
set -e

export GOOS=linux
export GOARCH=arm64

go test ./...

go build -tags lambda.norpc -ldflags="-s -w -X 'github.com/m4x1202/collaborative-book/internal/app/version.Version=0.1.1'" -o bin/main cmd/lambda/main.go
zip --junk-paths bin/main.zip bin/main
aws lambda update-function-code --function-name collaborative-book --zip-file fileb://bin/main.zip
