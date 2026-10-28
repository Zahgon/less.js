package less

import (
	"strings"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// Combinator is tree/combinator.js.
type Combinator struct {
	nodeBase
	value             string
	emptyOrWhitespace bool
}

func newCombinator(v string) *Combinator {
	if v == " " {
		return &Combinator{value: " ", emptyOrWhitespace: true}
	}
	c := &Combinator{value: js.Trim(v)}
	c.emptyOrWhitespace = c.value == ""
	return c
}

func (c *Combinator) Type() string { return "Combinator" }

func (c *Combinator) cmpValue() any { return c.value }

func (c *Combinator) genCSS(ctx *evalContext, out *output) {
	space := " "
	if ctx.compress || c.value == "" || c.value == " " || c.value == "|" {
		space = ""
	}
	out.add(space + c.value + space)
}

// Element is tree/element.js; its value is a string (valueStr) or a node.
type Element struct {
	nodeBase
	combinator *Combinator
	value      Node
	valueStr   string
	isVariable bool
}

func newElementStr(c *Combinator, s string, isVariable bool, index int, fi *FileInfo) *Element {
	return newElementVis(c, nil, js.Trim(s), isVariable, index, fi, nil)
}

func newElementNode(c *Combinator, n Node, isVariable bool, index int, fi *FileInfo) *Element {
	return newElementVis(c, n, "", isVariable, index, fi, nil)
}

func newElementVis(c *Combinator, n Node, s string, isVariable bool, index int, fi *FileInfo, vis *visibilityInfo) *Element {
	if c == nil {
		c = newCombinator("")
	}
	e := &Element{combinator: c, value: n, valueStr: s, isVariable: isVariable}
	e.index = index
	e.fileInfo = fi
	e.copyVisibilityInfo(vis)
	setParent(c, e)
	return e
}

func (e *Element) Type() string { return "Element" }

func (e *Element) cmpValue() any {
	if e.value != nil {
		return e.value
	}
	return e.valueStr
}

func (e *Element) accept(v visitor) {
	if c, ok := v.visit(e.combinator).(*Combinator); ok {
		e.combinator = c
	}
	if e.value != nil {
		e.value = v.visit(e.value)
	}
}

func (e *Element) eval(ctx *evalContext) Node {
	if e.value != nil {
		return newElementVis(e.combinator, evalNode(e.value, ctx), "", e.isVariable, getIndex(e), fileInfoOf(e), e.visibilityInfo())
	}
	return newElementVis(e.combinator, nil, e.valueStr, e.isVariable, getIndex(e), fileInfoOf(e), e.visibilityInfo())
}

func (e *Element) clone() *Element {
	return newElementVis(e.combinator, e.value, e.valueStr, e.isVariable, getIndex(e), fileInfoOf(e), e.visibilityInfo())
}

func (e *Element) genCSS(ctx *evalContext, out *output) {
	out.addAt(e.toCSS(ctx), fileInfoOf(e), getIndex(e), false)
}

func (e *Element) toCSS(ctx *evalContext) string {
	if ctx == nil {
		ctx = &evalContext{}
	}
	first := ctx.firstSelector
	value := e.valueStr
	if e.value != nil {
		if _, ok := e.value.(*Paren); ok {
			ctx.firstSelector = true
		}
		value = toCSS(e.value, ctx)
	}
	ctx.firstSelector = first
	if value == "" && strings.HasPrefix(e.combinator.value, "&") {
		return ""
	}
	return toCSS(e.combinator, ctx) + value
}

// isParentRef reports whether the element is `&`.
func (e *Element) isParentRef() bool { return e.value == nil && e.valueStr == "&" }

// Selector is tree/selector.js. Elements are usually *Element, but a parenthesised selector list holds nested
// selectors and comma Anonymous nodes.
type Selector struct {
	nodeBase
	elements       []Node
	extendList     []Node
	condition      Node
	evaldCondition bool
	mixinEls       []string
	hasMixinEls    bool
	mediaEmpty     bool
}

func newSelector(elements []Node, extendList []Node, condition Node, index int, fi *FileInfo, vis *visibilityInfo) *Selector {
	s := &Selector{extendList: extendList, condition: condition, evaldCondition: condition == nil}
	s.index = index
	s.fileInfo = fi
	s.elements = s.getElements(elements)
	s.copyVisibilityInfo(vis)
	setParents(s.elements, s)
	return s
}

func (s *Selector) Type() string { return "Selector" }

func (s *Selector) cmpValue() any { return nil }

func (s *Selector) accept(v visitor) {
	if s.elements != nil {
		s.elements = v.visitArray(s.elements)
	}
	if s.extendList != nil {
		s.extendList = v.visitArray(s.extendList)
	}
	if s.condition != nil {
		s.condition = v.visit(s.condition)
	}
}

func (s *Selector) createDerived(elements []Node, extendList []Node, evaldCondition *bool) *Selector {
	elements = s.getElements(elements)
	if extendList == nil {
		extendList = s.extendList
	}
	n := newSelector(elements, extendList, nil, getIndex(s), fileInfoOf(s), s.visibilityInfo())
	if evaldCondition != nil {
		n.evaldCondition = *evaldCondition
	} else {
		n.evaldCondition = s.evaldCondition
	}
	n.mediaEmpty = s.mediaEmpty
	return n
}

func (s *Selector) getElements(els []Node) []Node {
	if els == nil {
		return []Node{newElementStr(newCombinator(""), "&", false, s.index, s.fileInfo)}
	}
	return els
}

func (s *Selector) createEmptySelectors() []*Selector {
	el := newElementStr(newCombinator(""), "&", false, s.index, s.fileInfo)
	sel := newSelector([]Node{el}, nil, nil, s.index, s.fileInfo, nil)
	sel.mediaEmpty = true
	return []*Selector{sel}
}

func (s *Selector) match(other *Selector) int {
	mixinEls := other.mixinElements()
	olen := len(mixinEls)
	if olen == 0 || len(s.elements) < olen {
		return 0
	}
	for i := 0; i < olen; i++ {
		if elementValueString(s.elements[i]) != mixinEls[i] {
			return 0
		}
	}
	return olen
}

var reMixinElements = jsre.MustCompile(`[,&#*.\w-]([\w-]|(\\.))*`, "g")

func (s *Selector) mixinElements() []string {
	if s.hasMixinEls {
		return s.mixinEls
	}
	var b strings.Builder
	for _, n := range s.elements {
		e, ok := n.(*Element)
		if !ok {
			typeError("Cannot read properties of undefined (reading 'value')")
		}
		b.WriteString(e.combinator.value)
		b.WriteString(elementInnerValueString(e))
	}
	els := reMixinElements.MatchAll(b.String())
	if len(els) > 0 && els[0] == "&" {
		els = els[1:]
	}
	if els == nil {
		els = []string{}
	}
	s.mixinEls, s.hasMixinEls = els, true
	return els
}

// elementValueString is `element.value` compared with ===: only a string value can equal a mixin element.
func elementValueString(n Node) string {
	if e, ok := n.(*Element); ok && e.value == nil {
		return e.valueStr
	}
	return "\x00object"
}

// elementInnerValueString is `(v.value.value || v.value)` coerced to a string.
func elementInnerValueString(e *Element) string {
	if e.value == nil {
		return e.valueStr
	}
	if s, ok := stringValue(e.value); ok && s != "" {
		return s
	}
	return "[object Object]"
}

func (s *Selector) isJustParentSelector() bool {
	if s.mediaEmpty || len(s.elements) != 1 {
		return false
	}
	e, ok := s.elements[0].(*Element)
	return ok && e.isParentRef() && (e.combinator.value == " " || e.combinator.value == "")
}

func (s *Selector) eval(ctx *evalContext) Node {
	var evald *bool
	if s.condition != nil {
		b := conditionValue(s.condition, ctx)
		evald = &b
	}
	var elements []Node
	if s.elements != nil {
		elements = make([]Node, len(s.elements))
		for i, e := range s.elements {
			elements[i] = evalNode(e, ctx)
		}
	}
	var extendList []Node
	if s.extendList != nil {
		extendList = make([]Node, len(s.extendList))
		for i, e := range s.extendList {
			extendList[i] = evalNode(e, ctx)
		}
	}
	return s.createDerived(elements, extendList, evald)
}

func (s *Selector) genCSS(ctx *evalContext, out *output) {
	if ctx == nil || !ctx.firstSelector {
		e, ok := s.elements[0].(*Element)
		if !ok {
			typeError("Cannot read properties of undefined (reading 'value')")
		}
		if e.combinator.value == "" {
			out.addAt(" ", fileInfoOf(s), getIndex(s), false)
		}
	}
	for _, e := range s.elements {
		genCSS(e, ctx, out)
	}
}

func (s *Selector) getIsOutput() bool { return s.evaldCondition }

func selectorNodes(sels []*Selector) []Node {
	out := make([]Node, len(sels))
	for i, s := range sels {
		out[i] = s
	}
	return out
}
