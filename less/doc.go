// Package less is the Less 4.9.1 compiler: it parses a stylesheet, evaluates it (variables, mixins, guards,
// operations, built-in functions, imports and extend) and generates CSS and source maps exactly as the original
// JavaScript implementation does, including the JavaScript number, string and regular expression semantics its
// output depends on. Render is the entry point; the lessc command in cmd/lessc is built on it.
package less
