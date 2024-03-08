go test ./...

$env:GOOS = "linux"
$env:CGO_ENABLED = "0"
$env:GOARCH = "arm64"

go build -tags lambda.norpc -ldflags="-s -w -X 'github.com/m4x1202/collaborative-book/internal/app/version.Version=0.1.1'" -o bin/main cmd/lambda/main.go
build-lambda-zip.exe -output bin/main.zip bin/main
aws lambda update-function-code --function-name collaborative-book --zip-file fileb://bin/main.zip
