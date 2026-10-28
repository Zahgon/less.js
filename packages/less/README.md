<p align="center"><img src="http://lesscss.org/public/img/less_logo.png" width="264" height="117" alt="Less.js logo"></p>

<p align="center">
    <a href="https://github.com/less/less.js/actions?query=branch%3Amaster"><img src="https://github.com/less/less.js/actions/workflows/ci.yml/badge.svg?branch=master" alt="Github Actions CI"/></a>
</p>

# Less.js

> The dynamic stylesheet language. [lesscss.org](http://lesscss.org)

Less extends CSS with variables, mixins, functions, nesting, and more — then compiles to standard CSS. Write cleaner stylesheets with less code.

```less
@primary: #4a90d9;

.button {
  color: @primary;
  &:hover {
    color: darken(@primary, 10%);
  }
}
```

## Install

```sh
go install github.com/less/less.js/cmd/lessc@latest
```

## Usage

### Go

```go
import "github.com/less/less.js/less"

res, err := less.Render(".class { width: (1 + 1) }", less.DefaultOptions())
if err != nil {
	log.Fatal(err.Format(false))
}
fmt.Print(res.CSS)
```

### Command Line

```sh
lessc styles.less styles.css
```

This directory keeps the source map expectations of the test suite (`test/sourcemaps*`); the Go code is at the
repository root.

## Why Less?

- **Variables** — define reusable values once
- **Mixins** — reuse groups of declarations across rulesets
- **Nesting** — mirror HTML structure in your stylesheets
- **Functions** — transform colors, manipulate strings, do math
- **Imports** — split stylesheets into manageable pieces
- **Extend** — reduce output size by combining selectors

## Documentation

Full documentation, usage guides, and configuration options at **[lesscss.org](http://lesscss.org)**.

## Contributing

Less.js is open source. [Report bugs](https://github.com/less/less.js/issues), submit pull requests, or help improve the [documentation](https://github.com/less/less-docs).

See [CONTRIBUTING.md](https://github.com/less/less.js/blob/master/CONTRIBUTING.md) for development setup (Go 1.23 or newer, `go test ./...`).

## License

Copyright (c) 2009-2025 [Alexis Sellier](http://cloudhead.io) & The Core Less Team
Licensed under the [Apache License](https://github.com/less/less.js/blob/master/LICENSE).
