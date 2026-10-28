package less

import (
	"strings"

	"github.com/less/less.js/internal/js"
)

// Declaration is tree/declaration.js. Before evaluation a property name is a list of nodes (nameNodes); a
// variable name and every evaluated name is a string.
type Declaration struct {
	nodeBase
	name      string
	nameNodes []Node
	value     Node
	important string
	merge     string
	inline    bool
	variable  bool
	parsed    bool
}

func newDeclaration(nameNodes []Node, name string, value Node, important, merge string, index int, fi *FileInfo, inline bool, variable *bool) *Declaration {
	d := &Declaration{name: name, nameNodes: nameNodes, value: value, merge: merge, inline: inline}
	if important != "" {
		d.important = " " + js.Trim(important)
	}
	d.index = index
	d.fileInfo = fi
	if variable != nil {
		d.variable = *variable
	} else {
		d.variable = nameNodes == nil && strings.HasPrefix(name, "@")
	}
	setParent(value, d)
	return d
}

// Type is the node's type name, as the original's type getter reports it.
func (d *Declaration) Type() string { return "Declaration" }

func (d *Declaration) cmpValue() any { return d.value }

// nameKey is the declaration name used as an object key (an unevaluated name list stringifies as objects).
func (d *Declaration) nameKey() string {
	if d.nameNodes != nil {
		return strings.TrimSuffix(strings.Repeat("[object Object],", len(d.nameNodes)), ",")
	}
	return d.name
}

func (d *Declaration) accept(v visitor) { d.value = v.visit(d.value) }

func (d *Declaration) genCSS(ctx *evalContext, out *output) {
	sep := ": "
	if ctx.compress {
		sep = ":"
	}
	out.addAt(d.nameKey()+sep, fileInfoOf(d), getIndex(d), false)
	func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(*errorInfo); ok {
					e.Index = intp(d.index)
					e.Filename = ""
					if d.fileInfo != nil {
						e.Filename = d.fileInfo.Filename
					}
				}
				panic(r)
			}
		}()
		genCSS(d.value, ctx, out)
	}()
	end := ";"
	if d.inline || (ctx.lastRule && ctx.compress) {
		end = ""
	}
	out.addAt(d.important+end, d.fileInfo, d.index, false)
}

func evalName(ctx *evalContext, name []Node) string {
	o := &output{}
	for _, n := range name {
		genCSS(evalNode(n, ctx), ctx, o)
	}
	return o.String()
}

func (d *Declaration) eval(ctx *evalContext) (result Node) {
	name := d.name
	variable := d.variable
	if d.nameNodes != nil {
		if k, ok := d.nameNodes[0].(*Keyword); ok && len(d.nameNodes) == 1 {
			name = k.value
		} else {
			name = evalName(ctx, d.nameNodes)
		}
		variable = false
	}
	mathBypass := false
	prevMath := ctx.math
	if name == "font" && ctx.math == MathAlways {
		mathBypass = true
		ctx.math = MathParensDivision
	}
	defer func() {
		if mathBypass {
			ctx.math = prevMath
		}
		if r := recover(); r != nil {
			if e, ok := r.(*errorInfo); ok && e.Index == nil {
				e.Index = intp(getIndex(d))
				e.Filename = fileInfoOf(d).Filename
			}
			panic(r)
		}
	}()
	ctx.importantScope = append(ctx.importantScope, &importantScope{})
	evald := evalNode(d.value, ctx)
	if !d.variable {
		switch evald.(type) {
		case *DetachedRuleset, *nodeList:
			panic(&errorInfo{Message: "Rulesets cannot be evaluated on a property.", Index: intp(getIndex(d)), Filename: fileInfoOf(d).Filename})
		}
	}
	important := d.important
	scope := ctx.importantScope[len(ctx.importantScope)-1]
	ctx.importantScope = ctx.importantScope[:len(ctx.importantScope)-1]
	if important == "" && scope.important != "" {
		important = scope.important
	}
	return newDeclaration(nil, name, evald, important, d.merge, getIndex(d), fileInfoOf(d), d.inline, &variable)
}

func (d *Declaration) makeImportant() Node {
	return newDeclaration(d.nameNodes, d.name, d.value, "!important", d.merge, getIndex(d), fileInfoOf(d), d.inline, nil)
}

// mergeRules is tree/merge-rules.js: declarations with a merge flag are folded into the first of their name.
func mergeRules(rules []Node) []Node {
	groups := map[string][]*Declaration{}
	var order []string
	out := rules[:0:0]
	for _, r := range rules {
		d, ok := r.(*Declaration)
		if !ok || d.merge == "" {
			out = append(out, r)
			continue
		}
		key := d.nameKey()
		if _, seen := groups[key]; seen {
			groups[key] = append(groups[key], d)
			continue
		}
		order = append(order, key)
		groups[key] = []*Declaration{d}
		out = append(out, r)
	}
	for _, key := range order {
		group := groups[key]
		result := group[0]
		var space []Node
		comma := []Node{}
		cur := newExpression(space, false)
		comma = append(comma, cur)
		for _, rule := range group {
			if rule.merge == "+" && len(cur.value) > 0 {
				cur = newExpression(nil, false)
				comma = append(comma, cur)
			}
			cur.value = append(cur.value, rule.value)
			if result.important == "" {
				result.important = rule.important
			}
		}
		result.value = newValue(comma, 0)
	}
	return out
}

func mergeDeclarations(ds []*Declaration) []*Declaration {
	nodes := make([]Node, len(ds))
	for i, d := range ds {
		nodes[i] = d
	}
	nodes = mergeRules(nodes)
	out := make([]*Declaration, len(nodes))
	for i, n := range nodes {
		out[i] = n.(*Declaration)
	}
	return out
}
