# Contributing to Less.js

Thank you for your interest in contributing to Less.js! Contributions come in many forms—fixing bugs, improving code quality, enhancing tooling, updating documentation, and occasionally adding new features. This guide will help you get started.

## Getting Started

Before you begin, please note: **Words that begin with the at sign (`@`) must be wrapped in backticks!** This prevents unintended notifications to GitHub users. For example, use `` `@username` `` instead of `@username`.

GitHub has many great markdown features—[learn more about them here](https://help.github.com/articles/github-flavored-markdown).

## Reporting Issues

We welcome bug reports and feature requests! To help us help you, please follow these guidelines:

1. **Search for existing issues first.** Many issues have already been reported or resolved. Checking first saves everyone time.
2. **Create an isolated and reproducible test case.** Include [reduced test cases](http://css-tricks.com/reduced-test-cases/) that demonstrate the problem clearly.
3. **Test with the latest version.** Many issues are resolved in newer versions—please update first.
4. **Include examples with source code.** You can use [Less Preview](http://lesscss.org/less-preview/) to create a short test case.
5. **Share as much information as possible.** Include:
   - Operating system and version
   - How you're using Less (browser, command line, build tool, etc.)
   - Browser and version (if applicable)
   - Version of Less.js you're using
   - Clear steps to reproduce the issue
6. **Suggest solutions if you have them.** If you know how to fix it, share your approach or submit a pull request!

Please report documentation issues in [the documentation project](https://github.com/less/less-docs).

## Feature Requests

When suggesting features:

* **Search existing feature requests first** to see if something similar already exists. Many features are already planned or under consideration.
* **Include a clear and specific use-case.** Help us understand the practical need and how it would be used.
* **Consider alternatives.** Sometimes a function or a 3rd-party build system might be a better fit than a core language feature.

**Note:** Most helpful contributions to Less.js are organizational—addressing bugs, improving code quality, enhancing tooling, and updating documentation. The language features are generally stable, even if not all planned features have been implemented yet.

## Pull Requests

Pull requests are welcome! Here's how to make them go smoothly:

* **For new features, start with a feature request** to get feedback and see how your idea is received.
* **If your PR solves an existing issue**, but approaches it differently, please create a new issue first and discuss it with core contributors. This helps avoid wasted effort.
* The `dist/` folder is gitignored—release builds of `lessc` happen automatically when a version tag is pushed.
* **Please add tests** for your work. Run them with `go test ./...`, which compiles every fixture of `packages/test-data` and checks the source maps, errors and warnings.

### Coding Standards

* Format with `gofmt` (CI fails on any file `gofmt -l .` lists)
* Keep `go vet ./...` clean
* Use the Go standard library only: the module has no requirements
* Return errors as values at the library and command boundary, and document exported names

## Developing

If you want to work on an issue, add a comment saying you're taking it on—this helps prevent duplicate work.

You need Go 1.23 or newer and nothing else:

```sh
go build -o lessc ./cmd/lessc     # build the command
go vet ./...                      # static checks
go test ./...                     # the fixture suite and the unit tests
gofmt -l .                        # lists any file that needs formatting
```

The compiler lives in `less/`, the command in `cmd/lessc`, and the JavaScript semantics the output depends on in
`internal/`. Fixtures and their expected output are in `packages/test-data`; each folder's options are in its
`styles.config.json`. See [.github/TESTING_PUBLISHING.md](.github/TESTING_PUBLISHING.md) for the test layout.

Learn more about the language at [lesscss.org](http://lesscss.org).

## Releases

A release is a version tag. Pushing `vX.Y.Z` (or `vX.Y.Z-alpha.N` for a pre-release) runs
`.github/workflows/release.yml`, which formats, vets and tests the module, builds `lessc` for Linux, macOS and
Windows on amd64 and arm64, and creates the GitHub release with the binaries and their `SHA256SUMS`.

1. Update `CHANGELOG.md` and the version `lessc --version` prints (`cmd/lessc/main.go`).
2. Merge into `master` (or `alpha` for a pre-release).
3. Tag the merge commit and push the tag: `git tag v4.9.2 && git push origin v4.9.2`.

Go users get the release with `go install github.com/less/less.js/cmd/lessc@v4.9.2`; the module proxy needs no
publishing step.

---

Thank you for contributing to Less.js!
