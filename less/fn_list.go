package less

import (
	"math"

	"github.com/less/less.js/internal/js"
)

// list functions (functions/list.js), type checks (types.js), logical functions (boolean.js) and style (style.js).

// itemsOf is getItemsFromNode: an array value, or the node itself as a one-item list.
func itemsOf(n Node) []Node {
	if n == nil {
		readUndefined("value")
	}
	if list, ok := arrayValue(n); ok {
		return list
	}
	return []Node{n}
}

func fnEach(fc *funcCaller, list, rs Node) Node {
	tryEval := func(n Node) Node { return evalNode(n, fc.ctx) }
	if list == nil {
		readUndefined("value")
	}
	var iterator []Node
	switch l := list.(type) {
	case *Expression:
		for _, n := range l.value {
			iterator = append(iterator, tryEval(n))
		}
	case *Value:
		for _, n := range l.value {
			iterator = append(iterator, tryEval(n))
		}
	case *nodeList:
		for _, n := range l.nodes {
			iterator = append(iterator, tryEval(n))
		}
	case *DetachedRuleset:
		iterator = l.ruleset.eval(fc.ctx).(*Ruleset).rules
	case *Ruleset:
		for _, n := range l.rules {
			iterator = append(iterator, tryEval(n))
		}
	default:
		if v, ok := valueNodeOf(list); ok {
			iterator = []Node{tryEval(v)}
			break
		}
		if s, ok := stringValue(list); ok && s != "" {
			if _, isQuoted := list.(*Quoted); !isQuoted {
				iterator = []Node{newValue([]Node{newAnonymous(s, 0, nil, false, false, nil)}, 0)}
				break
			}
		}
		if d, ok := list.(*Dimension); ok && d.value != 0 {
			iterator = []Node{newValue([]Node{newAnonymous(js.NumberToString(d.value), 0, nil, false, false, nil)}, 0)}
			break
		}
		iterator = []Node{tryEval(list)}
	}
	valueName, keyName, indexName := "@value", "@key", "@index"
	var body *Ruleset
	switch r := rs.(type) {
	case nil:
		readUndefined("params")
	case *MixinDefinition:
		valueName, keyName, indexName = paramName(r.params, 0), paramName(r.params, 1), paramName(r.params, 2)
		body = r.drRules
		if body == nil {
			body = newRuleset(nil, r.rules, false, nil)
		}
	case *DetachedRuleset:
		body = r.ruleset
	default:
		readUndefined("rules")
	}
	var rules []Node
	for i, item := range iterator {
		var key, value Node
		if d, ok := item.(*Declaration); ok {
			if d.nameNodes != nil {
				key = newAnonymous(jsValueString(d.nameNodes[0]), 0, nil, false, false, nil)
			} else {
				key = newAnonymous(d.name, 0, nil, false, false, nil)
			}
			value = d.value
		} else {
			key = newDimension(float64(i+1), nil)
			value = item
		}
		if _, ok := item.(*Comment); ok {
			continue
		}
		newRules := append([]Node{}, body.rules...)
		if valueName != "" {
			newRules = append(newRules, newDeclaration(nil, valueName, value, "", "", fc.index, fc.fileInfo, false, nil))
		}
		if indexName != "" {
			newRules = append(newRules, newDeclaration(nil, indexName, newDimension(float64(i+1), nil), "", "", fc.index, fc.fileInfo, false, nil))
		}
		if keyName != "" {
			newRules = append(newRules, newDeclaration(nil, keyName, key, "", "", fc.index, fc.fileInfo, false, nil))
		}
		rules = append(rules, newRuleset([]*Selector{ampSelector()}, newRules, body.strictImports, body.visibilityInfo()))
	}
	return newRuleset([]*Selector{ampSelector()}, rules, body.strictImports, body.visibilityInfo()).eval(fc.ctx)
}

func ampSelector() *Selector {
	return newSelector([]Node{newElementStr(newCombinator(""), "&", false, 0, nil)}, nil, nil, 0, nil, nil)
}

func paramName(params []mixinArg, i int) string {
	if i < len(params) {
		return params[i].name
	}
	return ""
}

func isa(ok bool) Node {
	if ok {
		return keywordTrue
	}
	return keywordFalse
}

func isunit(n, unit Node) Node {
	if unit == nil {
		argumentError("missing the required second argument to isunit.")
	}
	u, ok := stringValue(unit)
	if !ok {
		if s, isStr := unit.(*jsString); isStr {
			u, ok = s.v, true
		}
	}
	if !ok {
		argumentError("Second argument to isunit should be a unit or a string.")
	}
	d, isDim := n.(*Dimension)
	return isa(isDim && d.unit.is(u))
}

// jsString is a plain JavaScript string argument (ispixel passes 'px' to isunit).
type jsString struct {
	nodeBase
	v string
}

func (s *jsString) Type() string { return "" }

func init() {
	type1 := func(test func(Node) bool) func(*funcCaller, []Node) Node {
		return func(fc *funcCaller, a []Node) Node { return isa(test(argAt(a, 0))) }
	}
	registerFunctions(map[string]func(*funcCaller, []Node) Node{
		"_self": func(fc *funcCaller, a []Node) Node { return argAt(a, 0) },
		"~": func(fc *funcCaller, a []Node) Node {
			if len(a) == 1 {
				return a[0]
			}
			return newValue(a, 0)
		},
		"extract": func(fc *funcCaller, a []Node) Node {
			idx := valueOf(argAt(a, 1)) - 1
			items := itemsOf(argAt(a, 0))
			if idx != math.Trunc(idx) || idx < 0 || int(idx) >= len(items) {
				return nil
			}
			return items[int(idx)]
		},
		"length": func(fc *funcCaller, a []Node) Node { return newDimension(float64(len(itemsOf(argAt(a, 0)))), nil) },
		"range": func(fc *funcCaller, a []Node) Node {
			start, end, step := argAt(a, 0), argAt(a, 1), argAt(a, 2)
			from, stepValue := 1.0, 1.0
			to := start
			if end != nil {
				to = end
				from = valueOf(start)
				if step != nil {
					stepValue = valueOf(step)
				}
			}
			limit := valueOf(to)
			var unit *Unit
			if d, ok := to.(*Dimension); ok {
				unit = d.unit
			}
			var list []Node
			for i := from; i <= limit; i += stepValue {
				list = append(list, newDimension(i, unit))
				if len(list) > 100000 {
					break
				}
			}
			return newExpression(list, false)
		},
		"each":      func(fc *funcCaller, a []Node) Node { return fnEach(fc, argAt(a, 0), argAt(a, 1)) },
		"isruleset": type1(func(n Node) bool { _, ok := n.(*DetachedRuleset); return ok }),
		"iscolor":   type1(func(n Node) bool { _, ok := n.(*Color); return ok }),
		"isnumber":  type1(func(n Node) bool { _, ok := n.(*Dimension); return ok }),
		"isstring":  type1(func(n Node) bool { _, ok := n.(*Quoted); return ok }),
		"iskeyword": type1(func(n Node) bool { _, ok := n.(*Keyword); return ok }),
		"isurl":     type1(func(n Node) bool { _, ok := n.(*URL); return ok }),
		"ispixel":   func(fc *funcCaller, a []Node) Node { return isunit(argAt(a, 0), &jsString{v: "px"}) },
		"ispercentage": func(fc *funcCaller, a []Node) Node {
			return isunit(argAt(a, 0), &jsString{v: "%"})
		},
		"isem":   func(fc *funcCaller, a []Node) Node { return isunit(argAt(a, 0), &jsString{v: "em"}) },
		"isunit": func(fc *funcCaller, a []Node) Node { return isunit(argAt(a, 0), argAt(a, 1)) },
		"unit": func(fc *funcCaller, a []Node) Node {
			val, unit := argAt(a, 0), argAt(a, 1)
			d, ok := val.(*Dimension)
			if !ok {
				msg := "the first argument to unit must be a number"
				if _, isOp := val.(*Operation); isOp {
					msg += ". Have you forgotten parenthesis?"
				}
				argumentError(msg)
			}
			u := ""
			if unit != nil {
				if k, isKw := unit.(*Keyword); isKw {
					u = k.value
				} else {
					u = toCSS(unit, nil)
				}
			}
			return newDimension(d.value, newUnit(unitList(u), nil, ""))
		},
		"get-unit": func(fc *funcCaller, a []Node) Node {
			n := argAt(a, 0)
			if n == nil {
				readUndefined("unit")
			}
			if d, ok := n.(*Dimension); ok {
				return &Anonymous{valueNode: d.unit}
			}
			return newAnonymous("", 0, nil, false, false, nil)
		},
		"boolean": func(fc *funcCaller, a []Node) Node { return isa(truthy(argAt(a, 0))) },
		"style": func(fc *funcCaller, a []Node) Node {
			return swallow(func() Node {
				if len(a) == 0 {
					argumentError("one or more arguments required")
				}
				name, _ := stringValue(a[0])
				v := newVariable(name, fc.index, fc.fileInfo).eval(fc.ctx)
				return newAnonymous("style("+toCSS(v, fc.ctx)+")", 0, nil, false, false, nil)
			})
		},
	})
	globalFunctions.add("if", &lessFunction{rawArgs: true, fn: func(fc *funcCaller, a []Node) Node {
		cond, t, f := argAt(a, 0), argAt(a, 1), argAt(a, 2)
		if cond == nil {
			readUndefined("eval")
		}
		if truthy(evalNode(cond, fc.ctx)) {
			if t == nil {
				readUndefined("eval")
			}
			return evalNode(t, fc.ctx)
		}
		if f != nil {
			return evalNode(f, fc.ctx)
		}
		return newAnonymous("", 0, nil, false, false, nil)
	}})
	globalFunctions.add("isdefined", &lessFunction{rawArgs: true, fn: func(fc *funcCaller, a []Node) Node {
		return isa(swallow(func() Node {
			v := argAt(a, 0)
			if v == nil {
				readUndefined("eval")
			}
			evalNode(v, fc.ctx)
			return keywordTrue
		}) != nil)
	}})
	globalFunctions.add("default", &lessFunction{fn: func(fc *funcCaller, a []Node) Node { return defaultFn.eval() }})
}
