GOOS=darwin GOARCH=arm64 go build -o ./npm/@rev-dep/darwin-arm64/bin/rev-dep -ldflags="-s -w" .
GOOS=linux GOARCH=amd64 go build -o ./npm/@rev-dep/linux-x64/bin/rev-dep -ldflags="-s -w" .