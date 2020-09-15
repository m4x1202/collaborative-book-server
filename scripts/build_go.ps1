$env:GOOS = "linux"
$env:CGO_ENABLED = "0"
$env:GOARCH = "amd64"

go test .\...

go build -ldflags="-s -w" -o bin\main cmd\lambda\main.go
build-lambda-zip.exe -output bin\main.zip bin\main
aws lambda update-function-code --function-name collaborative-book --zip-file fileb://bin\main.zip
