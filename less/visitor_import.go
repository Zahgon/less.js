package less

// importVisitor is visitors/import-visitor.js with import-sequencer.js. The original loads files asynchronously;
// here loads complete in request order from a queue, and the sequencer runs the callbacks in import order.
type importVisitor struct {
	v           *Visitor
	importer    *importManager
	ctx         *evalContext
	importCount int
	onceFiles   map[string]bool
	recursion   map[string]bool
	err         *errorInfo
	isFinished  bool
	seq         *importSequencer
}

type sequencedImport struct {
	callback func()
	ready    bool
}

type importSequencer struct {
	imports         []*sequencedImport
	variableImports []func()
	depth           int
}

func (s *importSequencer) addImport(cb func(args func())) func(func()) {
	item := &sequencedImport{}
	s.imports = append(s.imports, item)
	return func(run func()) {
		item.callback = run
		item.ready = true
		s.tryRun()
	}
}

func (s *importSequencer) tryRun() {
	s.depth++
	defer func() { s.depth-- }()
	for {
		for len(s.imports) > 0 {
			item := s.imports[0]
			if !item.ready {
				return
			}
			s.imports = s.imports[1:]
			item.callback()
		}
		if len(s.variableImports) == 0 {
			break
		}
		next := s.variableImports[0]
		s.variableImports = s.variableImports[1:]
		next()
	}
}

func newImportVisitor(importer *importManager) *importVisitor {
	iv := &importVisitor{importer: importer, ctx: &evalContext{mathOn: true}, onceFiles: map[string]bool{},
		recursion: map[string]bool{}, seq: &importSequencer{}}
	iv.v = newVisitor(iv)
	return iv
}

// run visits the root, then drains the load queue; it returns the first error, as the finish callback receives.
func (iv *importVisitor) run(root Node) *errorInfo {
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(*errorInfo)
				if !ok {
					panic(r)
				}
				iv.err = e
			}
		}()
		iv.v.visit(root)
	}()
	iv.isFinished = true
	iv.seq.tryRun()
	iv.importer.drain()
	return iv.err
}

func (iv *importVisitor) isReplacing() bool { return false }

func (iv *importVisitor) visitIn(n Node, args *visitArgs) (any, bool) {
	switch x := n.(type) {
	case *Import:
		inline := isTrue(x.options.inline)
		if !x.css || inline {
			ctx := iv.ctx.copyEval(append([]Node{}, iv.ctx.frames...))
			var parent Node
			if len(ctx.frames) > 0 {
				parent = ctx.frames[0]
			}
			iv.importCount++
			if x.isVariableImport() {
				iv.seq.variableImports = append(iv.seq.variableImports, func() { iv.processImportNode(x, ctx, parent) })
			} else {
				iv.processImportNode(x, ctx, parent)
			}
		}
		args.visitDeeper = false
	case *Declaration:
		if _, ok := x.value.(*DetachedRuleset); ok {
			iv.ctx.frames = append([]Node{x}, iv.ctx.frames...)
		} else {
			args.visitDeeper = false
		}
	case *AtRule:
		switch {
		case x.value != nil:
			iv.ctx.frames = append([]Node{x}, iv.ctx.frames...)
		case len(x.declarations) > 0:
			if x.isRooted {
				iv.ctx.frames = append([]Node{x}, iv.ctx.frames...)
			} else {
				iv.ctx.frames = append([]Node{x.declarations[0]}, iv.ctx.frames...)
			}
		case len(x.rules) > 0:
			iv.ctx.frames = append([]Node{x}, iv.ctx.frames...)
		}
	case *MixinDefinition, *Ruleset:
		iv.ctx.frames = append([]Node{n}, iv.ctx.frames...)
	case *Media:
		iv.ctx.frames = append([]Node{x.rules[0]}, iv.ctx.frames...)
	default:
		return nil, false
	}
	return nil, true
}

func (iv *importVisitor) visitOut(n any) {
	switch x := n.(type) {
	case *Declaration:
		if _, ok := x.value.(*DetachedRuleset); ok {
			iv.ctx.frames = iv.ctx.frames[1:]
		}
	case *AtRule, *MixinDefinition, *Ruleset, *Media:
		if len(iv.ctx.frames) > 0 {
			iv.ctx.frames = iv.ctx.frames[1:]
		}
	}
}

func (iv *importVisitor) processImportNode(node *Import, ctx *evalContext, parent Node) {
	inline := isTrue(node.options.inline)
	var evald *Import
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(*errorInfo)
				if !ok {
					panic(r)
				}
				if e.Filename == "" {
					e.Index = intp(getIndex(node))
					e.Filename = fileInfoOf(node).Filename
				}
				node.css = true
				node.err = e
			}
		}()
		evald = node.evalForImport(ctx)
	}()
	if evald == nil || (evald.css && !inline) {
		iv.importCount--
		if iv.isFinished {
			iv.seq.tryRun()
		}
		return
	}
	if isTrue(evald.options.multiple) {
		ctx.importMultiple = true
	}
	tryAppend := !evald.cssDefined
	if rs, ok := parent.(*Ruleset); ok {
		for i, r := range rs.rules {
			if r == Node(node) {
				rs.rules[i] = evald
				break
			}
		}
	} else if md, ok := parent.(*MixinDefinition); ok {
		for i, r := range md.rules {
			if r == Node(node) {
				md.rules[i] = evald
				break
			}
		}
	}
	done := iv.seq.addImport(nil)
	path, _ := evald.getPath()
	iv.importer.push(path, tryAppend, fileInfoOf(evald), evald.options, func(e *errorInfo, root Node, text *string, atRoot bool, fullPath string) {
		done(func() { iv.onImported(evald, ctx, e, root, text, atRoot, fullPath) })
	})
}

func (iv *importVisitor) onImported(node *Import, ctx *evalContext, e *errorInfo, root Node, text *string, atRoot bool, fullPath string) {
	if e != nil {
		if e.Filename == "" {
			e.Index = intp(getIndex(node))
			e.Filename = fileInfoOf(node).Filename
		}
		iv.err = e
	}
	inline, isPlugin, optional := isTrue(node.options.inline), node.options.isPlugin, isTrue(node.options.optional)
	duplicate := atRoot || iv.recursion[fullPath]
	if !ctx.importMultiple {
		if duplicate {
			t := true
			node.skipped = &t
		} else {
			node.skip = func() bool {
				if iv.onceFiles[fullPath] {
					return true
				}
				iv.onceFiles[fullPath] = true
				return false
			}
		}
	}
	if fullPath == "" && optional {
		t := true
		node.skipped = &t
	}
	if root != nil || text != nil {
		node.root = root
		node.rootText = text
		node.importedFilename = fullPath
		if !inline && !isPlugin && root != nil && (ctx.importMultiple || !duplicate) {
			iv.recursion[fullPath] = true
			old := iv.ctx
			iv.ctx = ctx
			func() {
				defer func() {
					if r := recover(); r != nil {
						e, ok := r.(*errorInfo)
						if !ok {
							panic(r)
						}
						iv.err = e
					}
				}()
				iv.v.visit(root)
			}()
			iv.ctx = old
		}
	}
	iv.importCount--
	if iv.isFinished {
		iv.seq.tryRun()
	}
}
