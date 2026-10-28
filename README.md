<p align="center"><img src="http://lesscss.org/public/img/less_logo.png" width="264" height="117"></p>

<p align="center">
    <a href="https://github.com/less/less.js/actions?query=branch%3Amaster"><img src="https://github.com/less/less.js/actions/workflows/ci.yml/badge.svg?branch=master" alt="Github Actions CI"/></a>
    <a href="https://twitter.com/lesstocss"><img alt="Twitter Follow" src="https://img.shields.io/twitter/follow/lesstocss.svg?style=flat-square" style="max-width:100%;" /></a>
</p>

This is Less, the CSS preprocessor, as a Go module: the `lessc` command and the compiler library behind it. It
compiles Less 4.9.1 stylesheets exactly as the JavaScript implementation did, with the same options, messages,
output files and exit status, and it needs nothing beyond the Go standard library.

## Installing and running

```sh
go install github.com/less/less.js/cmd/lessc@latest   # or: go build -o lessc ./cmd/lessc
lessc styles.less styles.css                          # compile to a file
lessc -x --source-map styles.less out/styles.css      # compressed, with out/styles.css.map
lessc - < styles.less                                 # read the stylesheet from standard input
lessc --help                                          # every option
```

The library is the `less` package:

```go
opts := less.DefaultOptions()
opts.Filename = "styles.less"
res, err := less.Render(source, opts)
if err != nil {
	fmt.Fprintln(os.Stderr, err.Format(false))
	return
}
fmt.Print(res.CSS)
```

Inline JavaScript (backtick expressions under `--js`) and JavaScript plugins cannot run in Go: a stylesheet that
uses them reports an error, and `@import` over `http` and `https` is not fetched.

## More information

For general information on the language, configuration options or usage visit [lesscss.org](http://lesscss.org).

Here are other resources for using Less:

* [stackoverflow.com][so] is a great place to get answers about Less.
* [Less.js Issues][issues] for reporting bugs


## Contributing
Please read [CONTRIBUTING.md](CONTRIBUTING.md). Add tests for any new or changed functionality. Format, vet and test
your code with the Go toolchain: `gofmt -l .`, `go vet ./...` and `go test ./...`.

### Reporting Issues

Before opening any issue, please search for existing issues and read the [Issue Guidelines](https://github.com/necolas/issue-guidelines), written by [Nicolas Gallagher](https://github.com/necolas). After that if you find a bug or would like to make feature request, [please open a new issue][issues].

Please report documentation issues in [the documentation project](https://github.com/less/less-docs).

### Development

The command is in `cmd/lessc`, the compiler in `less/` (parser, tree nodes, visitors, built-in functions, source
maps), and the JavaScript runtime semantics the output depends on (number printing, UTF-16 strings, regular
expressions, URI and JSON encoding) in `internal/`. The shared fixtures are in `packages/test-data`; see
[.github/TESTING_PUBLISHING.md](.github/TESTING_PUBLISHING.md).

## Authors and licence

Less was created by Alexis Sellier (<self@cloudhead.net>) and is maintained by the Core Less Team. It is licensed
under the Apache License 2.0; see [LICENSE](LICENSE).

## Release History
See the [changelog](CHANGELOG.md)

## Contributors

This project exists thanks to all the people who contribute. [[Contribute](CONTRIBUTING.md)].
<a href="https://github.com/less/less.js/graphs/contributors"><img src="https://opencollective.com/less/contributors.svg?width=890&button=false" /></a>

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/matthew-dean"><img src="https://avatars.githubusercontent.com/u/414752?v=4?s=100" width="100px;" alt="Matthew Dean"/><br /><sub><b>Matthew Dean</b></sub></a><br /><a href="https://github.com/The Less CSS Team/Less.js/commits?author=matthew-dean" title="Code">💻</a> <a href="https://github.com/The Less CSS Team/Less.js/commits?author=matthew-dean" title="Documentation">📖</a> <a href="#maintenance-matthew-dean" title="Maintenance">🚧</a> <a href="#projectManagement-matthew-dean" title="Project Management">📆</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://cloudhead.io/"><img src="https://avatars.githubusercontent.com/u/40774?v=4?s=100" width="100px;" alt="Alexis Sellier"/><br /><sub><b>Alexis Sellier</b></sub></a><br /><a href="https://github.com/The Less CSS Team/Less.js/commits?author=cloudhead" title="Code">💻</a> <a href="https://github.com/The Less CSS Team/Less.js/commits?author=cloudhead" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/lukeapage"><img src="https://avatars.githubusercontent.com/u/309321?v=4?s=100" width="100px;" alt="Luke Page"/><br /><sub><b>Luke Page</b></sub></a><br /><a href="https://github.com/The Less CSS Team/Less.js/commits?author=lukeapage" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/seven-phases-max"><img src="https://avatars.githubusercontent.com/u/5304376?v=4?s=100" width="100px;" alt="Max Mikhailov"/><br /><sub><b>Max Mikhailov</b></sub></a><br /><a href="https://github.com/The Less CSS Team/Less.js/commits?author=seven-phases-max" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/iChenLei"><img src="https://avatars.githubusercontent.com/u/14012511?v=4?s=100" width="100px;" alt="Lei Chen"/><br /><sub><b>Lei Chen</b></sub></a><br /><a href="https://github.com/The Less CSS Team/Less.js/commits?author=iChenLei" title="Code">💻</a> <a href="https://github.com/The Less CSS Team/Less.js/issues?q=author%3AiChenLei" title="Bug reports">🐛</a> <a href="https://github.com/The Less CSS Team/Less.js/commits?author=iChenLei" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/puckowski"><img src="https://avatars.githubusercontent.com/u/3059609?v=4?s=100" width="100px;" alt="Daniel Puckowski"/><br /><sub><b>Daniel Puckowski</b></sub></a><br /><a href="https://github.com/The Less CSS Team/Less.js/commits?author=puckowski" title="Code">💻</a> <a href="https://github.com/The Less CSS Team/Less.js/issues?q=author%3Apuckowski" title="Bug reports">🐛</a></td>
    </tr>
  </tbody>
  <tfoot>
    <tr>
      <td align="center" size="13px" colspan="7">
        <img src="https://raw.githubusercontent.com/all-contributors/all-contributors-cli/1b8533af435da9854653492b1327a23a4dbd0a10/assets/logo-small.svg">
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Add your contributions</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->


## [License](LICENSE)

Copyright (c) 2009-2025 [Alexis Sellier](http://cloudhead.io) & The Core Less Team
Licensed under the [Apache License](LICENSE).


[so]: http://stackoverflow.com/questions/tagged/less "StackOverflow.com"
[issues]: https://github.com/less/less.js/issues "GitHub Issues for Less.js"
[download]: https://github.com/less/less.js/zipball/master "Download Less.js"
