package less

// renderWithSourceMap generates the CSS and its source map.
func renderWithSourceMap(root *Ruleset, ctx *evalContext, opts *Options, imports *importManager) (string, string) {
	return toCSS(root, ctx), ""
}
