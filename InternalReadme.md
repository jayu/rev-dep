## Build native development

`go build -tags "dev" -o rev-dep-go .`

## Build native production

`go build -o rev-dep-go .`

## Build native production without debug info in binary (smaller size)

`go build -o rev-dep-go -ldflags="-s -w" .`

## Build linux

`GOOS=linux GOARCH=amd64 go build -o rev-dep-go-linux .`

`GOOS=linux GOARCH=amd64 go build -o rev-dep-go-linux -ldflags="-s -w" .`


## CPU profiling 

`go test -run=None -bench=BenchmarkParseImportsWithTypes600Loc --cpuprofile prof.cpu`
`go tool pprof -http=":8081" rev-dep-go.test prof.cpu`

## Publishing

`node scripts/setVersions.js`

`scripts/buildProdBinaries.sh`

`node scripts/addCliDocsToReadme.js`

`scripts/publish.sh` 