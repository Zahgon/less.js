package less

// stringValue returns a node's `value` property when it is a JavaScript string.
func stringValue(n Node) (string, bool) {
	switch v := n.(type) {
	case *Quoted:
		return v.value, true
	case *Keyword:
		return v.value, true
	case *Anonymous:
		if v.valueNode == nil {
			return v.value, true
		}
	case *Comment:
		return v.value, true
	case *UnicodeDescriptor:
		return v.value, true
	case *Combinator:
		return v.value, true
	case *Color:
		if v.value != "" {
			return v.value, true
		}
	case *Element:
		if v.value == nil {
			return v.valueStr, true
		}
	}
	return "", false
}

// setStringValue assigns a node's `value` property, as URL.eval does on the evaluated node.
func setStringValue(n Node, s string) {
	switch v := n.(type) {
	case *Quoted:
		v.value = s
	case *Keyword:
		v.value = s
	case *Anonymous:
		v.value, v.valueNode = s, nil
	case *Comment:
		v.value = s
	case *UnicodeDescriptor:
		v.value = s
	case *Combinator:
		v.value = s
	case *Color:
		v.value = s
	case *Element:
		v.valueStr, v.value = s, nil
	}
}

// failStringValue raises the TypeError a string method call raises on a node value that is not a string.
func failStringValue(n Node, method string) { failStringValueAs(n, "path", method) }

// failStringValueAs is failStringValue for a value held in the named JavaScript variable.
func failStringValueAs(n Node, variable, method string) {
	switch n.(type) {
	case *Dimension, *Expression, *Value, *Anonymous, *URL, *Assignment, *Declaration, *Paren, *Negative,
		*Attribute, *Element, *NamespaceValue:
		typeError(variable + "." + method + " is not a function")
	}
	typeError("Cannot read properties of undefined (reading '" + method + "')")
}
