package less

import (
	"math"
	"strconv"
	"strings"

	"github.com/less/less.js/internal/js"
)

// FileInfo describes the file a node was parsed from.
type FileInfo struct {
	Filename         string
	Rootpath         string
	CurrentDirectory string
	RootFilename     string
	EntryPath        string
	Reference        bool
}

// Node is any tree node. Behaviour beyond the base is provided through the optional interfaces below, the way
// the JavaScript nodes override the methods of tree/node.js.
type Node interface {
	base() *nodeBase
	Type() string
}

type evaler interface{ eval(ctx *evalContext) Node }

type cssGenerator interface {
	genCSS(ctx *evalContext, out *output)
}

type acceptor interface{ accept(v visitor) }

type comparer interface{ compare(other Node) cmpResult }

type rulesetLike interface{ isRulesetLike() bool }

// cmpResult is the result of compare: -1, 0, 1, or cmpNone for "not comparable" (undefined in JavaScript).
type cmpResult int

const cmpNone cmpResult = -99

// triBool is a JavaScript property that may be undefined, false or true.
type triBool int8

const (
	triUndefined triBool = iota
	triFalse
	triTrue
)

func (t triBool) truthy() bool { return t == triTrue }

type nodeBase struct {
	noSpacing        triBool // Expression.eval copies noSpacing onto any node it returns
	parent           Node
	index            int
	fileInfo         *FileInfo
	visibilityBlocks *int
	nodeVisible      *bool
}

func (b *nodeBase) base() *nodeBase { return b }

func getIndex(n Node) int {
	for n != nil {
		b := n.base()
		if b.index != 0 {
			return b.index
		}
		n = b.parent
	}
	return 0
}

func fileInfoOf(n Node) *FileInfo {
	for n != nil {
		b := n.base()
		if b.fileInfo != nil {
			return b.fileInfo
		}
		n = b.parent
	}
	return &FileInfo{}
}

func setParent(n Node, parent Node) {
	if n != nil {
		n.base().parent = parent
	}
}

func setParents(ns []Node, parent Node) {
	for _, n := range ns {
		setParent(n, parent)
	}
}

func evalNode(n Node, ctx *evalContext) Node {
	if e, ok := n.(evaler); ok {
		return e.eval(ctx)
	}
	return n
}

func genCSS(n Node, ctx *evalContext, out *output) {
	if g, ok := n.(cssGenerator); ok {
		g.genCSS(ctx, out)
	}
}

func toCSS(n Node, ctx *evalContext) string {
	o := &output{}
	genCSS(n, ctx, o)
	return o.String()
}

func isRulesetLikeNode(n Node) bool {
	r, ok := n.(rulesetLike)
	return ok && r.isRulesetLike()
}

func (b *nodeBase) blocksVisibility() bool {
	if b.visibilityBlocks == nil {
		z := 0
		b.visibilityBlocks = &z
	}
	return *b.visibilityBlocks != 0
}

func (b *nodeBase) addVisibilityBlock() {
	if b.visibilityBlocks == nil {
		z := 0
		b.visibilityBlocks = &z
	}
	v := *b.visibilityBlocks + 1
	b.visibilityBlocks = &v
}

func (b *nodeBase) removeVisibilityBlock() {
	if b.visibilityBlocks == nil {
		z := 0
		b.visibilityBlocks = &z
	}
	v := *b.visibilityBlocks - 1
	b.visibilityBlocks = &v
}

func (b *nodeBase) ensureVisibility() { t := true; b.nodeVisible = &t }

func (b *nodeBase) ensureInvisibility() { f := false; b.nodeVisible = &f }

// isVisible returns nodeVisible, which may be undefined (nil).
func (b *nodeBase) isVisible() *bool { return b.nodeVisible }

type visibilityInfo struct {
	visibilityBlocks *int
	nodeVisible      *bool
}

func (b *nodeBase) visibilityInfo() *visibilityInfo {
	return &visibilityInfo{visibilityBlocks: b.visibilityBlocks, nodeVisible: b.nodeVisible}
}

func (b *nodeBase) copyVisibilityInfo(info *visibilityInfo) {
	if info == nil {
		return
	}
	b.visibilityBlocks = info.visibilityBlocks
	b.nodeVisible = info.nodeVisible
}

func isTrue(b *bool) bool { return b != nil && *b }

func operate(op string, a, b float64) float64 {
	switch op {
	case "+":
		return a + b
	case "-":
		return a - b
	case "*":
		return a * b
	case "/":
		return a / b
	}
	return math.NaN()
}

// fround is Number((value + 2e-16).toFixed(precision)) when a precision is set.
func fround(ctx *evalContext, value float64) float64 {
	if ctx == nil || ctx.numPrecision == 0 {
		return value
	}
	s, err := js.ToFixed(value+2e-16, float64(ctx.numPrecision))
	if err != nil {
		panic(err)
	}
	return js.StringToNumber(s)
}

// nodeValuer is implemented by nodes whose `value` takes part in Node.compare.
type nodeValuer interface{ cmpValue() any }

func compareNodes(a, b Node) cmpResult {
	if ac, ok := a.(comparer); ok && !(b.Type() == "Quoted" || b.Type() == "Anonymous") {
		return ac.compare(b)
	} else if bc, ok := b.(comparer); ok {
		r := bc.compare(a)
		if r == cmpNone {
			return cmpNone
		}
		return -r
	} else if a.Type() != b.Type() {
		return cmpNone
	}
	av, aok := a.(nodeValuer)
	bv, bok := b.(nodeValuer)
	if !aok || !bok {
		return cmpNone
	}
	aVal, bVal := av.cmpValue(), bv.cmpValue()
	aList, aIsList := aVal.([]Node)
	if !aIsList {
		if aVal == bVal {
			return 0
		}
		return cmpNone
	}
	bList, bIsList := bVal.([]Node)
	if !bIsList || len(aList) != len(bList) {
		return cmpNone
	}
	for i := range aList {
		if compareNodes(aList[i], bList[i]) != 0 {
			return cmpNone
		}
	}
	return 0
}

func numericCompare(a, b float64) cmpResult {
	switch {
	case a < b:
		return -1
	case a == b:
		return 0
	case a > b:
		return 1
	}
	return cmpNone
}

func stringCompare(a, b string) cmpResult {
	switch {
	case lessU16(a, b):
		return -1
	case a == b:
		return 0
	default:
		return 1
	}
}

// lessU16 compares strings by UTF-16 code units, as JavaScript's < does.
func lessU16(a, b string) bool {
	ua, ub := js.ToU16(a), js.ToU16(b)
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// output collects generated CSS; the source map builder hooks add to record mappings.
type output struct {
	strs  []string
	hook  func(chunk string, fi *FileInfo, index int, mapLines bool)
	empty func() bool
}

func (o *output) add(chunk string) { o.addAt(chunk, nil, 0, false) }

func (o *output) addAt(chunk string, fi *FileInfo, index int, mapLines bool) {
	if o.hook != nil {
		o.hook(chunk, fi, index, mapLines)
		return
	}
	o.strs = append(o.strs, chunk)
}

// addObject adds a chunk that is an object rather than a string: joined output prints its toString, while the
// source map output fails on it the way chunk.split fails on a non-string.
func (o *output) addObject(text string, fi *FileInfo, index int, mapLines bool) {
	if o.hook != nil {
		typeError("chunk.split is not a function")
	}
	o.strs = append(o.strs, text)
}

func (o *output) isEmpty() bool {
	if o.empty != nil {
		return o.empty()
	}
	return len(o.strs) == 0
}

// String returns the generated CSS.
func (o *output) String() string { return strings.Join(o.strs, "") }

func itoa(i int) string { return strconv.Itoa(i) }
