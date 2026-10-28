package less

import (
	"github.com/less/less.js/internal/js"
)

// lessFunction is a built-in function. Functions with evalArgs false receive the context and raw arguments.
type lessFunction struct {
	fn      func(fc *funcCaller, args []Node) Node
	rawArgs bool
}

// functionRegistry is functions/function-registry.js.
type functionRegistry struct {
	data map[string]*lessFunction
	base *functionRegistry
}

func newFunctionRegistry(base *functionRegistry) *functionRegistry {
	return &functionRegistry{data: map[string]*lessFunction{}, base: base}
}

func (r *functionRegistry) add(name string, f *lessFunction) { r.data[js.ToLowerCase(name)] = f }

func (r *functionRegistry) get(name string) *lessFunction {
	if f := r.data[name]; f != nil {
		return f
	}
	if r.base != nil {
		return r.base.get(name)
	}
	return objectPrototypeFunction(name)
}

func (r *functionRegistry) inherit() *functionRegistry { return newFunctionRegistry(r) }

// objectPrototypeFunction returns what the registry's plain-object lookup finds on Object.prototype for the
// lower-case names it can see: `constructor` is Object, and `__proto__` is an object that cannot be called.
func objectPrototypeFunction(name string) *lessFunction {
	switch name {
	case "constructor":
		return &lessFunction{fn: func(fc *funcCaller, args []Node) Node {
			if len(args) > 0 {
				return args[0]
			}
			return newAnonymous("[object Object]", 0, nil, false, false, nil)
		}}
	case "__proto__":
		return &lessFunction{fn: func(fc *funcCaller, args []Node) Node {
			typeError("this.func is not a function")
			return nil
		}}
	}
	return nil
}

var globalFunctions = newFunctionRegistry(nil)

func registerFunctions(fs map[string]func(fc *funcCaller, args []Node) Node) {
	for name, f := range fs {
		globalFunctions.add(name, &lessFunction{fn: f})
	}
}

// funcCaller is functions/function-caller.js; it is also `this` inside a function.
type funcCaller struct {
	name     string
	ctx      *evalContext
	index    int
	fileInfo *FileInfo
	fn       *lessFunction
}

func newFuncCaller(name string, ctx *evalContext, index int, fi *FileInfo) *funcCaller {
	fc := &funcCaller{name: js.ToLowerCase(name), ctx: ctx, index: index, fileInfo: fi}
	if len(ctx.frames) == 0 {
		typeError("Cannot read properties of undefined (reading 'functionRegistry')")
	}
	fr, ok := ctx.frames[0].(frameNode)
	if !ok || fr.registry() == nil {
		typeError("Cannot read properties of undefined (reading 'get')")
	}
	fc.fn = fr.registry().get(fc.name)
	return fc
}

func (fc *funcCaller) isValid() bool { return fc.fn != nil }

func (fc *funcCaller) call(args []Node) Node {
	if !fc.fn.rawArgs {
		evald := make([]Node, len(args))
		for i, a := range args {
			if a == nil {
				typeError("Cannot read properties of undefined (reading 'eval')")
			}
			evald[i] = evalNode(a, fc.ctx)
		}
		args = evald
	}
	var filtered []Node
	for _, item := range args {
		if item == nil {
			typeError("Cannot read properties of undefined (reading 'type')")
		}
		if item.Type() == "Comment" {
			continue
		}
		if e, ok := item.(*Expression); ok {
			var sub []Node
			for _, n := range e.value {
				if n.Type() != "Comment" {
					sub = append(sub, n)
				}
			}
			if len(sub) == 1 {
				if op, isOp := sub[0].(*Operation); e.parens && isOp && op.op == "/" {
					filtered = append(filtered, item)
				} else {
					filtered = append(filtered, sub[0])
				}
			} else {
				filtered = append(filtered, newExpression(sub, false))
			}
			continue
		}
		filtered = append(filtered, item)
	}
	return fc.fn.fn(fc, filtered)
}

// defaultFunc is functions/default.js: a module-level state shared by every evaluation.
type defaultFunc struct {
	value *bool
	err   *errorInfo
}

var defaultFn = &defaultFunc{}

func (d *defaultFunc) eval() Node {
	if d.err != nil {
		e := *d.err
		panic(&e)
	}
	if d.value != nil {
		if *d.value {
			return keywordTrue
		}
		return keywordFalse
	}
	return nil
}

func (d *defaultFunc) setValue(v bool) { d.value = &v }

func (d *defaultFunc) setError(e *errorInfo) { d.err = e }

func (d *defaultFunc) reset() { d.value, d.err = nil, nil }
