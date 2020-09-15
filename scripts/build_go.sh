#!/usr/bin/env bash

go build -o bin/main server.go
zip --junk-paths bin/main.zip bin/main
aws lambda update-function-code --function-name collaborative-book --zip-file fileb://bin/main.zip
