package less

// Import is tree/import.js.
type Import struct {
	nodeBase
	options          *importOptions
	path             Node
	features         Node
	css              bool
	cssDefined       bool // css was set by the constructor (it stays undefined otherwise)
	layerCss         bool
	root             Node // the imported file's root Ruleset
	rootText         *string
	importedFilename string
	skip             func() bool
	skipped          *bool
	err              *errorInfo
}

var reCSSImportPath = re(`[#.&?]css([?;].*)?$`)

func newImport(path, features Node, opts *importOptions, index int, fi *FileInfo) *Import {
	return newImportVis(path, features, opts, index, fi, nil)
}

func newImportVis(path, features Node, opts *importOptions, index int, fi *FileInfo, vis *visibilityInfo) *Import {
	im := &Import{options: opts, path: path, features: features}
	im.index = index
	im.fileInfo = fi
	if opts.less != nil || isTrue(opts.inline) {
		im.css = !isTrue(opts.less) || isTrue(opts.inline)
		im.cssDefined = true
	} else if p, ok := im.getPath(); ok && p != "" && reCSSImportPath.Test(p) {
		im.css = true
		im.cssDefined = true
	}
	im.copyVisibilityInfo(vis)
	if features != nil {
		setParent(features, im)
	}
	setParent(path, im)
	return im
}

func (im *Import) Type() string { return "Import" }

func (im *Import) accept(v visitor) {
	if im.features != nil {
		im.features = v.visit(im.features)
	}
	im.path = v.visit(im.path)
	if !im.options.isPlugin && !isTrue(im.options.inline) && im.root != nil {
		im.root = v.visit(im.root)
	}
}

func (im *Import) pathFileInfo() *FileInfo { return im.path.base().fileInfo }

func (im *Import) genCSS(ctx *evalContext, out *output) {
	fi := im.pathFileInfo()
	if fi == nil {
		typeError("Cannot read properties of undefined (reading 'reference')")
	}
	if im.css && !fi.Reference {
		out.addAt("@import ", im.fileInfo, im.index, false)
		genCSS(im.path, ctx, out)
		if im.features != nil {
			out.add(" ")
			genCSS(im.features, ctx, out)
		}
		out.add(";")
	}
}

// getPath returns the path string (the URL's inner value for url()).
func (im *Import) getPath() (string, bool) {
	if u, ok := im.path.(*URL); ok {
		return stringValue(u.value)
	}
	return stringValue(im.path)
}

func (im *Import) isVariableImport() bool {
	path := im.path
	if u, ok := path.(*URL); ok {
		path = u.value
	}
	if q, ok := path.(*Quoted); ok {
		return q.containsVariables()
	}
	return true
}

func (im *Import) evalForImport(ctx *evalContext) *Import {
	path := im.path
	if u, ok := path.(*URL); ok {
		path = u.value
	}
	return newImportVis(evalNode(path, ctx), im.features, im.options, im.index, im.fileInfo, im.visibilityInfo())
}

func (im *Import) evalPath(ctx *evalContext) Node {
	path := evalNode(im.path, ctx)
	if _, isURL := path.(*URL); !isURL {
		s, isStr := stringValue(path)
		if im.fileInfo != nil && isStr && s != "" && ctx.pathRequiresRewrite(s) {
			setStringValue(path, ctx.rewritePath(s, im.fileInfo.Rootpath))
		} else {
			if !isStr {
				failStringValue(path, "split")
			}
			setStringValue(path, normalizePath(s))
		}
	}
	return path
}

// evalImport is Import.eval; a list result is returned as a nodeList.
func (im *Import) evalImport(ctx *evalContext) Node {
	result := im.doEval(ctx)
	if isTrue(im.options.reference) || im.blocksVisibility() {
		if list, ok := result.(*nodeList); ok {
			for _, n := range list.nodes {
				n.base().addVisibilityBlock()
			}
		} else {
			result.base().addVisibilityBlock()
		}
	}
	return result
}

func (im *Import) eval(ctx *evalContext) Node { return im.evalImport(ctx) }

func layerFeature(list []Node) bool {
	if len(list) < 2 {
		return false
	}
	k, ok := list[0].(*Keyword)
	return ok && k.value == "layer" && list[1].Type() == "Paren"
}

func (im *Import) doEval(ctx *evalContext) Node {
	var features Node
	if im.features != nil {
		features = evalNode(im.features, ctx)
	}
	if im.options.isPlugin {
		return &nodeList{}
	}
	if im.skip != nil || im.skipped != nil {
		if im.skipped == nil {
			s := im.skip()
			im.skipped = &s
		}
		if *im.skipped {
			return &nodeList{}
		}
	}
	featureList := func() ([]Node, bool) {
		if im.features == nil {
			return nil, false
		}
		return arrayValue(im.features)
	}
	if fv, ok := featureList(); ok && len(fv) >= 1 {
		if e, isExpr := fv[0].(*Expression); isExpr && len(e.value) >= 2 && layerFeature(e.value) {
			im.css = false
		}
	}
	switch {
	case isTrue(im.options.inline):
		text := ""
		if im.rootText != nil {
			text = *im.rootText
		}
		ref := false
		if fi := im.pathFileInfo(); fi != nil {
			ref = fi.Reference
		}
		contents := newAnonymous(text, 0, &FileInfo{Filename: im.importedFilename, Reference: ref}, true, true, nil)
		if im.features != nil {
			fv, _ := arrayValue(im.features)
			return newMedia([]Node{contents}, fv, 0, nil, nil)
		}
		return &nodeList{nodes: []Node{contents}}
	case im.css || im.layerCss:
		n := newImportVis(im.evalPath(ctx), features, im.options, im.index, nil, nil)
		if im.layerCss {
			n.css = im.layerCss
			n.path.base().fileInfo = im.fileInfo
		}
		if !n.css && im.err != nil {
			panic(im.err)
		}
		return n
	case im.root != nil:
		if fv, ok := featureList(); ok && len(fv) == 1 {
			if e, isExpr := fv[0].(*Expression); isExpr && len(e.value) >= 2 && layerFeature(e.value) {
				im.layerCss = true
				joined := newExpression(append([]Node{}, e.value[:2]...), false)
				joined.noSpacing = true
				e.value = append([]Node{joined}, e.value[2:]...)
				return im
			}
		}
		root := im.root.(*Ruleset)
		rs := newRuleset(nil, append([]Node{}, root.rules...), false, nil)
		rs.evalImports(ctx)
		if im.features != nil {
			fv, _ := arrayValue(im.features)
			return newMedia(rs.rules, fv, 0, nil, nil)
		}
		return &nodeList{nodes: rs.rules}
	default:
		if fv, ok := featureList(); ok && len(fv) >= 1 {
			if inner, isList := arrayValue(fv[0]); isList && len(inner) >= 2 && layerFeature(inner) {
				im.css = true
				joined := newExpression(append([]Node{}, inner[:2]...), false)
				joined.noSpacing = true
				setArrayValue(fv[0], append([]Node{joined}, inner[2:]...))
				return im
			}
		}
		return &nodeList{}
	}
}

func setArrayValue(n Node, v []Node) {
	switch x := n.(type) {
	case *Expression:
		x.value = v
	case *Value:
		x.value = v
	}
}
