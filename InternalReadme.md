## Build native development

`go build -tags "dev" -o rev-dep-go-dev .`

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

`node scripts/addCliRefToReadme.js`

`git add . && git commit -m "chore: release`

`npm login`

`scripts/publish.sh` 

`git push origin HEAD` 

## Resolution steps contribution to overall performance

Performance measurements for GetMinimalDepsTreeForCwd:
Task                                          Duration      %
----                                          --------      ---
CreateGlobMatchers                            1.541µs       0.00%
FindAndProcessGitIgnoreFilesUpToRepoRoot      157.625µs     0.03%
GetFiles                                      98.988709ms   21.22%
ParseImportsFromFiles                         117.011542ms  25.08%
slices.Sort                                   271.708µs     0.06%
ResolveImports                                247.694292ms  53.09%
TransformToMinimalDependencyTreeCustomParser  2.335667ms    0.50%
resolverManager.CollectAllNodeModules         47.875µs      0.01%
Sum of measurements: 466.508959ms
Overall execution time: 466.513917ms

Build graph for all entry points 176.21725ms (that additional, not included in percetages above)
Optimized Build graph for all entry points ~ 60ms