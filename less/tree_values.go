package less

import (
	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// Keyword is tree/keyword.js.
type Keyword struct {
	nodeBase
	value string
}

func newKeyword(v string) *Keyword { return &Keyword{value: v} }

// Type is the node's type name, as the original's type getter reports it.
func (k *Keyword) Type() string { return "Keyword" }

func (k *Keyword) cmpValue() any { return k.value }

func (k *Keyword) genCSS(ctx *evalContext, out *output) {
	if k.value == "%" {
		panic(&errorInfo{Type: "Syntax", Message: "Invalid % without number"})
	}
	out.add(k.value)
}

var (
	keywordTrue  = newKeyword("true")
	keywordFalse = newKeyword("false")
)

// Anonymous is tree/anonymous.js; its value is a string, or a node in a few call sites.
type Anonymous struct {
	nodeBase
	value          string
	valueNode      Node
	mapLines       bool
	rulesetLike    bool
	preventReparse bool
}

func newAnonymous(value string, index int, fi *FileInfo, mapLines, rulesetLike bool, vis *visibilityInfo) *Anonymous {
	a := &Anonymous{value: value, mapLines: mapLines, rulesetLike: rulesetLike}
	a.index = index
	a.fileInfo = fi
	a.copyVisibilityInfo(vis)
	return a
}

// Type is the node's type name, as the original's type getter reports it.
func (a *Anonymous) Type() string { return "Anonymous" }

func (a *Anonymous) cmpValue() any {
	if a.valueNode != nil {
		return a.valueNode
	}
	return a.value
}

func (a *Anonymous) eval(ctx *evalContext) Node {
	n := newAnonymous(a.value, a.index, a.fileInfo, a.mapLines, a.rulesetLike, a.visibilityInfo())
	n.valueNode = a.valueNode
	n.preventReparse = a.preventReparse
	return n
}

func (a *Anonymous) compare(other Node) cmpResult {
	if toCSS(a, &evalContext{}) == toCSS(other, &evalContext{}) {
		return 0
	}
	return cmpNone
}

func (a *Anonymous) isRulesetLike() bool { return a.rulesetLike }

func (a *Anonymous) accept(v visitor) {
	if a.valueNode != nil {
		a.valueNode = v.visit(a.valueNode)
	}
}

func (a *Anonymous) genCSS(ctx *evalContext, out *output) {
	visible := a.value != "" || a.valueNode != nil
	a.nodeVisible = &visible
	if visible {
		if a.valueNode == nil {
			out.addAt(a.value, a.fileInfo, a.index, a.mapLines)
			return
		}
		s := "[object Object]"
		if u, ok := a.valueNode.(*Unit); ok {
			s = u.String()
		}
		out.addObject(s, a.fileInfo, a.index, a.mapLines)
	}
}

// Comment is tree/comment.js.
type Comment struct {
	nodeBase
	value         string
	isLineComment bool
	debugInfo     *debugInfo
}

func newComment(value string, isLineComment bool, index int, fi *FileInfo) *Comment {
	c := &Comment{value: value, isLineComment: isLineComment}
	c.index = index
	c.fileInfo = fi
	return c
}

// Type is the node's type name, as the original's type getter reports it.
func (c *Comment) Type() string { return "Comment" }

func (c *Comment) cmpValue() any { return c.value }

func (c *Comment) genCSS(ctx *evalContext, out *output) {
	if c.debugInfo != nil {
		out.addAt(debugInfoText(ctx, c.debugInfo, ""), fileInfoOf(c), getIndex(c), false)
	}
	out.add(c.value)
}

func (c *Comment) isSilent(ctx *evalContext) bool {
	compressed := ctx.compress && js.Slice16(c.value, 2, 3) != "!"
	return c.isLineComment || compressed
}

// debugInfoText is tree/debug-info.js.
func debugInfoText(ctx *evalContext, info *debugInfo, lineSeparator string) string {
	if ctx.dumpLineNumbers == "" || ctx.compress || info == nil {
		return ""
	}
	switch ctx.dumpLineNumbers {
	case "comments":
		return debugAsComment(info)
	case "mediaquery":
		return debugAsMediaQuery(info)
	case "all":
		return debugAsComment(info) + lineSeparator + debugAsMediaQuery(info)
	}
	return ""
}

func debugAsComment(info *debugInfo) string {
	return "/* line " + itoa(info.lineNumber) + ", " + info.fileName + " */\n"
}

var (
	reProtocol     = rei(`^[a-z]+:\/\/`)
	reDebugEscapes = jsre.MustCompile(`([.:/\\])`, "g")
)

func debugAsMediaQuery(info *debugInfo) string {
	name := info.fileName
	if !reProtocol.Test(name) {
		name = "file://" + name
	}
	escaped := reDebugEscapes.ReplaceFunc(name, func(a string, _ []*string) string {
		if a == "\\" {
			a = "/"
		}
		return "\\" + a
	})
	return "@media -sass-debug-info{filename{font-family:" + escaped + "}line{font-family:\\00003" + itoa(info.lineNumber) + "}}\n"
}

// UnicodeDescriptor is tree/unicode-descriptor.js.
type UnicodeDescriptor struct {
	nodeBase
	value string
}

func newUnicodeDescriptor(v string) *UnicodeDescriptor { return &UnicodeDescriptor{value: v} }

// Type is the node's type name, as the original's type getter reports it.
func (u *UnicodeDescriptor) Type() string { return "UnicodeDescriptor" }

func (u *UnicodeDescriptor) cmpValue() any { return u.value }

func (u *UnicodeDescriptor) genCSS(ctx *evalContext, out *output) { out.add(u.value) }

// Value is tree/value.js: a comma separated list.
type Value struct {
	nodeBase
	value []Node
}

func newValue(v []Node, _ int) *Value { return &Value{value: v} }

// Type is the node's type name, as the original's type getter reports it.
func (v *Value) Type() string { return "Value" }

func (v *Value) cmpValue() any { return v.value }

func (v *Value) accept(vis visitor) {
	if v.value != nil {
		v.value = vis.visitArray(v.value)
	}
}

func (v *Value) eval(ctx *evalContext) Node {
	if len(v.value) == 1 {
		return evalNode(v.value[0], ctx)
	}
	out := make([]Node, len(v.value))
	for i, n := range v.value {
		out[i] = evalNode(n, ctx)
	}
	return newValue(out, 0)
}

func (v *Value) genCSS(ctx *evalContext, out *output) {
	for i, n := range v.value {
		genCSS(n, ctx, out)
		if i+1 < len(v.value) {
			if ctx != nil && ctx.compress {
				out.add(",")
			} else {
				out.add(", ")
			}
		}
	}
}

// Quoted is tree/quoted.js.
type Quoted struct {
	nodeBase
	escaped       bool
	value         string
	quote         string
	variableRegex *jsre.Regexp
	propRegex     *jsre.Regexp
	allowRoot     bool
}

func newQuoted(str, content string, escaped bool, index int, fi *FileInfo) *Quoted {
	q := &Quoted{escaped: escaped, value: content, quote: js.Slice16(str, 0, 1),
		variableRegex: reVariableInterpAll, propRegex: rePropertyInterpAll, allowRoot: escaped}
	q.index = index
	q.fileInfo = fi
	return q
}

// Type is the node's type name, as the original's type getter reports it.
func (q *Quoted) Type() string { return "Quoted" }

func (q *Quoted) cmpValue() any { return q.value }

func (q *Quoted) genCSS(ctx *evalContext, out *output) {
	if !q.escaped {
		out.addAt(q.quote, fileInfoOf(q), getIndex(q), false)
	}
	out.add(q.value)
	if !q.escaped {
		out.add(q.quote)
	}
}

func (q *Quoted) containsVariables() bool { return q.variableRegex.Test(q.value) }

func firstSet(groups []*string) string {
	for _, g := range groups {
		if g != nil {
			return *g
		}
	}
	return ""
}

func (q *Quoted) eval(ctx *evalContext) Node {
	replace := func(value string, r *jsre.Regexp, resolve func(string) Node) string {
		evaluated := value
		for {
			value = evaluated
			evaluated = r.ReplaceFunc(value, func(_ string, groups []*string) string {
				v := evalNode(resolve(firstSet(groups)), ctx)
				if vq, ok := v.(*Quoted); ok {
					return vq.value
				}
				return toCSS(v, ctx)
			})
			if value == evaluated {
				return evaluated
			}
		}
	}
	value := replace(q.value, q.variableRegex, func(name string) Node {
		return resolveInterpolatedVariable(name, getIndex(q), fileInfoOf(q))
	})
	value = replace(value, q.propRegex, func(name string) Node {
		return newProperty("$"+name, getIndex(q), fileInfoOf(q))
	})
	r := newQuoted(q.quote+value+q.quote, value, q.escaped, getIndex(q), fileInfoOf(q))
	r.quote = q.quote
	return r
}

func (q *Quoted) compare(other Node) cmpResult {
	if o, ok := other.(*Quoted); ok && !q.escaped && !o.escaped {
		return stringCompareNumeric(q.value, o.value)
	}
	if toCSS(q, &evalContext{}) == toCSS(other, &evalContext{}) {
		return 0
	}
	return cmpNone
}

// stringCompareNumeric is Node.numericCompare on two strings.
func stringCompareNumeric(a, b string) cmpResult {
	switch {
	case lessU16(a, b):
		return -1
	case a == b:
		return 0
	case lessU16(b, a):
		return 1
	}
	return cmpNone
}
