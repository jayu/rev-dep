## Build native development

`go build -tags "dev" -o rev-dep-go-dev ./cmd/cli`

## Build native production

`go build -o rev-dep-go ./cmd/cli`

## Build native production without debug info in binary (smaller size)

`go build -o rev-dep-go -ldflags="-s -w" ./cmd/cli `

## Build linux

`GOOS=linux GOARCH=amd64 go build -o rev-dep-go-linux ./cmd/cli `

`GOOS=linux GOARCH=amd64 go build -o rev-dep-go-linux -ldflags="-s -w" ./cmd/cli `


## CPU profiling 

`go test -run=None -bench=BenchmarkParseImportsWithTypes600Loc --cpuprofile prof.cpu`
`go tool pprof -http=":8081" rev-dep-go.test prof.cpu`

## Publishing

> One-time setup: copy `cli-telemetry.env.example` to `cli-telemetry.env` (git-ignored) and
> fill in the connection string. `scripts/buildProdBinaries.sh` reads it, bakes
> it into the binaries, and fails the build if it is missing or did not link in — so a release can
> never ship with telemetry silently disabled.

1. `node scripts/setVersions.js <version>`   (e.g. `2.15.0`)
2. `scripts/buildProdBinaries.sh`
3. `node scripts/addCliRefToReadmeAndDocs.js`
4. `git add . && git commit -m "chore: release <version>"`
5. `npm login`
6. `scripts/publish.sh`
7. `node scripts/release.js`   — tag + GitHub release (notes, binaries, checksums)
8. `git push origin HEAD`

`scripts/release.js` reads the version from `npm/rev-dep/package.json`, pushes the
`<version>` tag, and creates a GitHub release with categorized notes (Features /
Bug Fixes / Documentation / Other Changes, built from the commits since the
previous tag - see `scripts/releaseNotes.js`), the three platform binaries, and a
`checksums.txt` (sha256). Requires `gh` authenticated. Pre-release versions
(`X.Y.Z-...`) are marked as pre-releases automatically.

### Verifying a downloaded binary

Download the binary and `checksums.txt` from the release page, place them in the
same folder, then:

```
shasum -a 256 -c checksums.txt
```

(Users installing via npm get integrity verification automatically from the
lockfile; the checksums are for people downloading the raw binary.)

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
