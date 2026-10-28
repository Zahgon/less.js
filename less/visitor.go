package less

// visitArgs is shared by every visit, like the module-level _visitArgs object of visitor.js.
type visitArgs struct{ visitDeeper bool }

var sharedVisitArgs = &visitArgs{}

// visitorImpl is a visitor implementation: visitIn is visit<Type> and reports whether the implementation has a
// function for the node's type; its result (a Node, a []Node or nil for undefined) replaces the node when the
// visitor is replacing. visitOut is visit<Type>Out.
type visitorImpl interface {
	isReplacing() bool
	visitIn(n Node, args *visitArgs) (any, bool)
	visitOut(n any)
}

// visitor is what a node's accept receives: the generic Visitor or the visibility visitor, which walks
// without visit<Type> dispatch.
type visitor interface {
	visit(n Node) Node
	visitAny(n Node) any
	visitArray(nodes []Node, nonReplacing ...bool) []Node
}

// Visitor walks a tree, calling the implementation for each node.
type Visitor struct {
	impl visitorImpl
}

func newVisitor(impl visitorImpl) *Visitor { return &Visitor{impl: impl} }

func (v *Visitor) visitAny(n Node) any {
	if n == nil {
		return nil
	}
	args := sharedVisitArgs
	args.visitDeeper = true
	var node any = n
	if res, handled := v.impl.visitIn(n, args); handled && v.impl.isReplacing() {
		node = res
	}
	if args.visitDeeper && node != nil {
		switch x := node.(type) {
		case []Node:
			for _, c := range x {
				if a, ok := c.(acceptor); ok {
					a.accept(v)
				}
			}
		case Node:
			if a, ok := x.(acceptor); ok {
				a.accept(v)
			}
		}
	}
	v.impl.visitOut(node)
	return node
}

// visit visits a node in a single-node slot.
func (v *Visitor) visit(n Node) Node {
	switch r := v.visitAny(n).(type) {
	case Node:
		return r
	case nil:
		return nil
	}
	return n
}

func (v *Visitor) visitArray(nodes []Node, nonReplacing ...bool) []Node {
	if nodes == nil {
		return nodes
	}
	if (len(nonReplacing) > 0 && nonReplacing[0]) || !v.impl.isReplacing() {
		for _, n := range nodes {
			v.visitAny(n)
		}
		return nodes
	}
	out := []Node{}
	for _, n := range nodes {
		switch r := v.visitAny(n).(type) {
		case nil:
		case []Node:
			if len(r) > 0 {
				out = flattenNodes(r, out)
			}
		case Node:
			out = append(out, r)
		}
	}
	return out
}

func flattenNodes(arr []Node, out []Node) []Node {
	for _, item := range arr {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}
