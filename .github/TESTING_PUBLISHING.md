# Testing and Publishing

This guide explains how the test suite is organised and how to check the release workflow before pushing a
version tag.

## The test suite

Everything runs offline with the Go toolchain:

```bash
go test ./...                          # every package
go test ./less -run TestCompileFixtures  # one set of fixtures
go test ./less -run 'TestErrorFixtures/tests-error/eval/' -v
```

The fixtures are the shared ones in `packages/test-data`, run by `less/*_test.go`:

| Test | Fixtures | Compared with |
|---|---|---|
| `TestCompileFixtures` | `tests-unit/*/*.less`, `tests-config/*/*.less`, `tests-config/debug/*/linenumbers-*.less` | the sibling `.css` |
| `TestErrorFixtures` | `tests-error/eval`, `tests-error/parse`, `tests-config/no-js-errors` | the sibling `.txt` (the error text) |
| `TestVariableFixtures` | `tests-config/globalVars`, `tests-config/modifyVars` with the variables of their `.json` | the sibling `.css` |
| `TestSourcemapFixtures` | `tests-config/sourcemaps*` | `packages/less/test/sourcemaps/*.json` |
| `TestSourcemapVariants` | `sourcemaps-empty`, `sourcemaps-disable-annotation`, `sourcemaps-variable-selector` | empty output, `packages/less/test/<set>/*.json` |
| `TestWarnings`, `TestDisablePluginRule`, `TestNoOptions` | inline stylesheets | the warnings and errors each must raise |

Each fixture runs with the options of the nearest `styles.config.json` at or above its folder (`language.less`:
`math`, `strictUnits`, `rewriteUrls`, `rootpath`, `sourceMap`, `globalVars` and the rest; a relative
`sourceMap.sourceMapBasepath` is resolved against the folder of the `styles.config.json`). Placeholders in the
expectations (`{path}`, `{pathesc}`, `{pathimport}`, `{pathrel}`) are replaced with the fixture's folder before
comparing. Fixtures that need a JavaScript plugin, inline JavaScript evaluation, an npm module or a network import
are skipped with the reason.

The packages under `internal/` have their own unit tests for the JavaScript semantics they reproduce.

## Testing the release workflow

`.github/workflows/release.yml` runs when a `v*` tag is pushed. Before tagging, run the same steps locally:

```bash
test -z "$(gofmt -l .)" && go vet ./... && go test ./...
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build -trimpath -o /dev/null ./cmd/lessc
done
```

To try the whole workflow without publishing to the real repository:

1. Push the branch to a fork; the job only runs when `github.repository` is `less/less.js`, so change that
   condition on the fork's copy of the workflow.
2. Push a throwaway pre-release tag such as `v0.0.0-alpha.1` to the fork.
3. Check the release the workflow creates: six `lessc` binaries and `SHA256SUMS`, marked as a pre-release.
4. Delete the tag and the release afterwards: `gh release delete v0.0.0-alpha.1 --cleanup-tag`.

## Pre-publish checklist

- [ ] `CHANGELOG.md` describes the release
- [ ] `lessc --version` prints the new version
- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `go test ./...` pass
- [ ] The tag is on the merge commit in `master` (or `alpha` for a pre-release)

**Never push a version tag to `less/less.js` unless you are ready to publish!**
