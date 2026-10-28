package less

import (
	"sort"
	"strconv"
	"strings"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

const (
	lookupKey           = `(?:[@$]{0,2})[_a-zA-Z0-9-]*`
	lookupChain         = `(?:\[` + lookupKey + `\])*`
	variableWithLookups = `[\w-]+` + lookupChain
)

func re(src string) *jsre.Regexp { return jsre.MustCompile(src, "") }

func rei(src string) *jsre.Regexp { return jsre.MustCompile(src, "i") }

var (
	reRulePropertyParticle = re(`^((?:[\w-]+)|(?:@\{` + variableWithLookups + `\})|(?:\$\{[\w-]+\}))`)
	reNumericVarName       = re(`^(?:@@?)?[0-9]`)
	reDashVarPrefix        = re(`^@@?`)
	rePluginOff            = re(`^@plugin?\s+`)
	reKeyword              = re(`^\[?(?:[\w-]|\\(?:[A-Fa-f0-9]{1,6} ?|[^A-Fa-f0-9]))+\]?`)
	reURLPeek              = rei(`^url\(`)
	reCallName             = re(`^([\w-]+|%|~|progid:[\w.]+)\(`)
	reDeclCall             = re(`^[\w]+\(`)
	reAtKeywordWord        = rei(`^(and|or|not|only|layer)$`)
	reAssignKey            = rei(`^\w+(?=\s?=)`)
	reURLBody              = re(`^(?:(?:\\[()'"])|[^()'"])+`)
	reVariableName         = re(`^@@?[\w-]+`)
	reSpaceStart           = re(`^\s`)
	reVariableCurly        = re(`^@\{([\w-]+)`)
	rePropertyName         = re(`^\$[\w-]+`)
	reColorHex             = re(`^#([A-Fa-f0-9]{8}|[A-Fa-f0-9]{6}|[A-Fa-f0-9]{3,4})([\w.#[])?`)
	reColorKeyword         = re(`^[_A-Za-z-][_A-Za-z0-9-]+`)
	reDimension            = rei(`^([+-]?\d*\.?\d+)(%|[a-z_]+)?`)
	reUnicodeDescriptor    = re(`^U\+[0-9a-fA-F?]+(-[0-9a-fA-F?]+)?`)
	reJavascript           = re("^[^`]*`")
	reVariableDef          = re(`^(@[\w-]+)\s*:`)
	reVariableCall         = re(`^(@[\w-]+)(\(\s*\))?`)
	reExtendAll            = re(`^(!?all)(?=\s*(\)|,))`)
	reCloseParen           = re(`^\)`)
	reSemicolon            = re(`^;`)
	reMixinElement         = re(`^[#.](?:[\w-]|\\(?:[A-Fa-f0-9]{1,6} ?|[^A-Fa-f0-9]))+`)
	reMixinDefPeek         = re(`^[^{]*\}`)
	reMixinDefName         = re(`^([#.](?:[\w-]|\\(?:[A-Fa-f0-9]{1,6} ?|[^A-Fa-f0-9]))+)\s*\(`)
	reLookupName           = re(`^(?:[@$]{0,2})[_a-zA-Z0-9-]*`)
	reOpacity              = rei(`^opacity=`)
	reDigits               = re(`^\d+`)
	reElementPercent       = re(`^(?:\d+\.\d+|\d+)%`)
	reElementName          = re(`^(?:[.#]?|:*)(?:[\w-]|[^\x00-\x9f]|\\(?:[A-Fa-f0-9]{1,6} ?|[^A-Fa-f0-9]))+`)
	reElementParen         = re(`^\([^&()@]+\)`)
	reElementInterpPrefix  = re(`^[.#:](?=@)`)
	reSlashedCombinator    = rei(`^\/[a-z]+\/`)
	reAttributeKey         = re(`^(?:[_A-Za-z0-9-*]*\|)?(?:[_A-Za-z0-9-]|\\.)+`)
	reAttributeOp          = re(`^[|~*$^]?=`)
	reAttributePercent     = re(`^[0-9]+%`)
	reWord                 = re(`^[\w-]+`)
	reAttributeFlag        = re(`^[iIsS]`)
	reDetachedArgs         = re(`^[.#]\(`)
	reAnonymousValue       = re(`^([^.#@$+/'"*` + "`" + `(;{}-]*);`)
	reBareVariable         = re(`@([\w-]+)`)
	reBareProperty         = re(`\$([\w-]+)`)
	reImport               = re(`^@import\s+`)
	reImportOption         = re(`^(less|css|multiple|once|inline|reference|optional)`)
	reCSSKeyword           = re(`^(?:--|-?(?:[_a-zA-Z0-9]|[^\0-\x7F]|\\[0-9a-fA-F]{1,6}\s?|\\[^\n\r\f0-9a-fA-F]))(?:[-_a-zA-Z0-9]|[^\0-\x7F]|\\[0-9a-fA-F]{1,6}\s?|\\[^\n\r\f0-9a-fA-F])*`)
	reCSSKeywordParen      = re(`^(?:--|-?(?:[_a-zA-Z0-9]|[^\0-\x7F]|\\[0-9a-fA-F]{1,6}\s?|\\[^\n\r\f0-9a-fA-F]))(?:[-_a-zA-Z0-9]|[^\0-\x7F]|\\[0-9a-fA-F]{1,6}\s?|\\[^\n\r\f0-9a-fA-F])*\s+\(`)
	reMediaLogicWord       = rei(`^(and|or|not|only)$`)
	reQueryRange           = re(`^(?:[^()]|\([^()]*\))*\s*([<>]=|<=|>=|[<>]|=)`)
	rePlugin               = re(`^@plugin\s+`)
	rePluginArgs           = re(`^\s*([^);]+)\)\s*`)
	reVariablePeek         = re(`^@@?[\w-]`)
	reAtRuleName           = re(`^@[a-z-]+`)
	reImportant            = re(`^! *important`)
	reColorOperand         = re(`^([lchrgbs])(?=\s|[,/*)]|$)`)
	reCommentPeek          = re(`^\/[*/]`)
	reAddOp                = re(`^[-+]\s+`)
	reConditionsPeek       = re(`^,\s*(not\s*)?\(`)
	reNegatePeek           = re(`^-[@$(]`)
	reProperty             = re(`^(\*?-?[_a-zA-Z0-9-]+)\s*:`)
	reSimpleProperty       = re(`^([_a-zA-Z0-9-]+)\s*:`)
	reStar                 = re(`^(\*?)`)
	reMergeColon           = re(`^((?:\+_|\+)?)\s*:`)
	reUntilSemiBrace       = re(`[;}]`)
	reUntilBraceSemi       = re(`^[{;]`)
	reVariableInterpAll    = jsre.MustCompile(`@\{(`+variableWithLookups+`)\}`, "g")
	rePropertyInterpAll    = jsre.MustCompile(`\$\{([\w-]+)\}`, "g")
	reHasVariableInterp    = re(`@\{(` + variableWithLookups + `)\}`)
	reHasPropertyInterp    = re(`\$\{([\w-]+)\}`)
)

// parseContext carries the options copied by contexts.Parse.
type parseContext struct {
	paths             []string
	rewriteUrls       int
	rootpath          string
	strictImports     bool
	insecure          bool
	dumpLineNumbers   string
	compress          bool
	syncImport        bool
	processImports    bool
	quiet             bool
	quietDeprecations bool
	// import bookkeeping
	importMultiple bool
}

type parser struct {
	ctx           *parseContext
	imports       *importManager
	fileInfo      *FileInfo
	currentIndex  int
	in            *parserInput
	warned        map[string]bool
	deprecations  map[string]int
	disablePlugin bool
}

func newParser(ctx *parseContext, imports *importManager, fi *FileInfo, currentIndex int) *parser {
	return &parser{ctx: ctx, imports: imports, fileInfo: fi, currentIndex: currentIndex, in: newParserInput(""),
		warned: map[string]bool{}, deprecations: map[string]int{}}
}

// files is the parser's imports as a LessError content map (nil when the parser has none).
func (p *parser) files() fileContents {
	if p.imports == nil {
		return nil
	}
	return p.imports
}

// context returns the parse context; reparsing a variable value runs without one, and reading it then fails
// with the TypeError the original raises.
func (p *parser) context(prop string) *parseContext {
	if p.ctx == nil {
		typeError("Cannot read properties of undefined (reading '" + prop + "')")
	}
	return p.ctx
}

func (p *parser) error(msg string, typ ...string) {
	t := "Syntax"
	if len(typ) > 0 && typ[0] != "" {
		t = typ[0]
	}
	panic(newLessError(&errorInfo{Index: intp(p.in.i), Filename: p.fileInfo.Filename, Type: t, Message: msg}, p.files(), ""))
}

// typeError raises a JavaScript TypeError the original throws from inside the parser.
func typeError(msg string) { panic(&errorInfo{Message: msg, jsError: true}) }

func (p *parser) warn(msg string, index int, typ, id string) {
	if p.context("quiet").quiet {
		return
	}
	if id != "" && p.ctx.quietDeprecations {
		return
	}
	if index < 0 {
		index = p.in.i
	}
	if id != "" {
		key := id + "@" + strconv.Itoa(index)
		if p.warned[key] {
			return
		}
		p.warned[key] = true
		p.deprecations[id]++
		if p.deprecations[id] > 5 {
			return
		}
	}
	t := "WARNING"
	if typ != "" {
		t = strings.ToUpper(typ) + " WARNING"
	}
	le := newLessError(&errorInfo{Index: intp(index), Filename: p.fileInfo.Filename, Type: t, Message: msg}, p.files(), "")
	lessLogger.warn(le.String(nil))
}

func (p *parser) warnBareAtRuleVariable(index int) {
	p.warn("A bare @variable in an at-rule prelude is deprecated. Use @{variable} interpolation instead.", index, "DEPRECATED", "variable-in-at-rule-prelude")
}

func isBareVariableReference(n Node) bool {
	if n == nil {
		return false
	}
	t := n.Type()
	return t == "Variable" || t == "VariableCall" || t == "NamespaceValue"
}

func (p *parser) warnNumericVariableName(name string, index int) {
	if reNumericVarName.Test(name) {
		p.warn("Variable names beginning with a number are deprecated and will be removed in Less 5.x. Rename the variable to start with a valid identifier character.", index, "DEPRECATED", "numeric-variable-name")
	}
}

func (p *parser) warnDashOnlyVariableName(name string, index int) {
	if reDashVarPrefix.Replace(name, "") == "-" {
		p.warn("The dash-only variable names @- and @{-} are deprecated and will be removed in Less 5.x. Rename the variable to use a valid identifier.", index, "DEPRECATED", "dash-only-variable-name")
	}
}

func (p *parser) warnDashOnlyMixinName(name string, index int) {
	if name == ".-" || name == "#-" {
		p.warn("The dash-only mixin names .-() and #-() are deprecated and will be removed in Less 5.x. Rename the mixin to use a valid CSS identifier.", index, "DEPRECATED", "dash-only-mixin-name")
	}
}

func (p *parser) warnDynamicCharset(value Node, index int) {
	dynamic := false
	switch v := value.(type) {
	case *Variable:
		dynamic = true
	case *Quoted:
		dynamic = v.containsVariables() || v.propRegex.Test(v.value)
	}
	if dynamic {
		p.warn("Dynamic @charset interpolation is deprecated and will be removed in Less 5.x. Use a static quoted encoding declaration instead.", index, "DEPRECATED", "dynamic-charset")
	}
}

func (p *parser) expectRe(r *jsre.Regexp, msg string) string {
	if s, ok := p.in.re(r); ok && s != "" {
		return s
	}
	if msg == "" {
		msg = "unexpected token"
	}
	p.error(msg)
	return ""
}

func (p *parser) expectNode(n Node, msg string) Node {
	if n != nil {
		return n
	}
	if msg == "" {
		msg = "unexpected token"
	}
	p.error(msg)
	return nil
}

func (p *parser) expectChar(c byte) {
	if p.in.char(c) {
		return
	}
	p.error("expected '" + string(c) + "' got '" + p.in.charAt(p.in.i) + "'")
}

type debugInfo struct {
	lineNumber int
	fileName   string
}

func (p *parser) getDebugInfo(index int) *debugInfo {
	line, _ := getLocation(intp(index), js.FromU16(p.in.input))
	return &debugInfo{lineNumber: *line + 1, fileName: p.fileInfo.Filename}
}

// parseNode parses str with the given productions; ok is false when the input was not consumed entirely.
func (p *parser) parseNode(str string, productions []string) (nodes []Node, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			e, isErr := r.(*errorInfo)
			if !isErr {
				panic(r)
			}
			idx := p.currentIndex
			if e.Index != nil {
				idx += *e.Index
			} else {
				panic(newLessError(&errorInfo{Message: e.Message}, p.files(), p.fileInfo.Filename))
			}
			panic(newLessError(&errorInfo{Index: intp(idx), Message: e.Message}, p.files(), p.fileInfo.Filename))
		}
	}()
	p.in = newParserInput(str)
	for _, prod := range productions {
		var n Node
		switch prod {
		case "value":
			n = nodeOrNil(p.value())
		case "important":
			if s := p.important(); s != "" {
				n = newKeyword(s)
			}
		case "selectors":
			if sels := p.selectors(); sels != nil {
				n = &selectorList{list: sels}
			}
		}
		nodes = append(nodes, n)
	}
	return nodes, p.in.end().isFinished
}

// selectorList carries the result of the selectors production through parseNode.
type selectorList struct {
	nodeBase
	list []*Selector
}

func (s *selectorList) Type() string { return "SelectorList" }

// nodeOrNil converts a possibly nil node pointer to a Node, keeping nil as a nil interface.
func nodeOrNil[T any, P interface {
	*T
	Node
}](n P) Node {
	if n == nil {
		return nil
	}
	return n
}

// serializeVars is Parser.serializeVars: keys in for-in order (array-index keys ascending, then insertion order).
func serializeVars(vars []VarOption) string {
	ordered := make([]VarOption, len(vars))
	copy(ordered, vars)
	sort.SliceStable(ordered, func(i, j int) bool {
		ai, aok := arrayIndexKey(ordered[i].Name)
		bi, bok := arrayIndexKey(ordered[j].Name)
		if aok && bok {
			return ai < bi
		}
		return aok && !bok
	})
	var b strings.Builder
	for _, v := range ordered {
		value := "undefined"
		if v.Value != nil {
			value = *v.Value
		}
		if !strings.HasPrefix(v.Name, "@") {
			b.WriteString("@")
		}
		b.WriteString(v.Name + ": " + value)
		if !strings.HasSuffix(value, ";") {
			b.WriteString(";")
		}
	}
	return b.String()
}

func arrayIndexKey(s string) (uint64, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n >= 4294967295 {
		return 0, false
	}
	return n, true
}

// parse is Parser.parse: it returns the root ruleset or the error the callback would receive.
func (p *parser) parse(str string, additional *Options) (root *Ruleset, err *errorInfo) {
	if additional != nil && additional.DisablePluginRule {
		p.disablePlugin = true
	}
	globalVars, modifyVars := "", ""
	if additional != nil && len(additional.GlobalVars) > 0 {
		globalVars = serializeVars(additional.GlobalVars) + "\n"
	}
	if additional != nil && len(additional.ModifyVars) > 0 {
		modifyVars = "\n" + serializeVars(additional.ModifyVars)
	}
	preText := ""
	if globalVars != "" {
		preText = globalVars
		p.imports.contentsIgnoredChars[p.fileInfo.Filename] += js.Len16(preText)
	}
	str = strings.ReplaceAll(strings.ReplaceAll(str, "\r\n", "\n"), "\r", "\n")
	str = preText + strings.TrimPrefix(str, "\uFEFF") + modifyVars
	p.imports.contents[p.fileInfo.Filename] = str
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(*errorInfo)
				if !ok {
					panic(r)
				}
				err = newLessError(e, p.imports, p.fileInfo.Filename)
			}
		}()
		p.in = newParserInput(str)
		root = newRuleset(nil, p.primary(), false, nil)
		root.root = true
		root.firstRoot = true
		root.functionRegistry = globalFunctions.inherit()
	}()
	if err != nil {
		return nil, err
	}
	end := p.in.end()
	if !end.isFinished && root != nil {
		message := end.furthestMessage
		if message == "" {
			message = "Unrecognised input"
			switch {
			case end.furthestChar == "}":
				message += ". Possibly missing opening '{'"
			case end.furthestChar == ")":
				message += ". Possibly missing opening '('"
			case end.furthestReachedEnd:
				message += ". Possibly missing something"
			}
		}
		return root, newLessError(&errorInfo{Type: "Parse", Message: message, Index: intp(end.furthest), Filename: p.fileInfo.Filename}, p.imports, "")
	}
	return root, nil
}

func (p *parser) primary() []Node {
	root := []Node{}
	for {
		for {
			n := p.comment()
			if n == nil {
				break
			}
			root = append(root, n)
		}
		if p.in.finished || p.in.peekStr("}") {
			break
		}
		if ext := p.extend(true); ext != nil {
			root = append(root, ext...)
			continue
		}
		n := p.mixinDefinition()
		if n == nil {
			n = p.declaration()
		}
		if n == nil {
			n = p.mixinCall(false, boolp(false))
		}
		if n == nil {
			n = p.ruleset()
		}
		if n == nil {
			n = p.variableCall("")
		}
		if n == nil {
			n = p.entCall()
		}
		if n == nil {
			n = p.atrule()
		}
		if n != nil {
			root = append(root, n)
			continue
		}
		found := false
		for p.in.char(';') {
			found = true
		}
		if !found {
			break
		}
	}
	return root
}

func boolp(b bool) *bool { return &b }

func (p *parser) comment() Node {
	if len(p.in.commentStore) == 0 {
		return nil
	}
	c := p.in.commentStore[0]
	p.in.commentStore = p.in.commentStore[1:]
	return newComment(c.text, c.isLineComment, c.index+p.currentIndex, p.fileInfo)
}

func (p *parser) entQuoted(forceEscaped bool) Node {
	index := p.in.i
	escaped := false
	p.in.save()
	if p.in.char('~') {
		escaped = true
	} else if forceEscaped {
		p.in.restore()
		return nil
	}
	str, ok := p.in.quoted()
	if !ok {
		p.in.restore()
		return nil
	}
	p.in.forget()
	n := js.Len16(str)
	return newQuoted(js.Slice16(str, 0, 1), js.Slice16(str, 1, n-1), escaped, index+p.currentIndex, p.fileInfo)
}

func (p *parser) entKeyword() Node {
	k := ""
	if p.in.char('%') {
		k = "%"
	} else if s, ok := p.in.re(reKeyword); ok {
		k = s
	}
	if k == "" {
		return nil
	}
	if c := colorFromKeyword(k); c != nil {
		return c
	}
	return newKeyword(k)
}

func (p *parser) entCall() Node {
	index := p.in.i
	if p.in.peekRe(reURLPeek) {
		return nil
	}
	p.in.save()
	m := p.in.match(reCallName)
	if m == nil {
		p.in.forget()
		return nil
	}
	name := *m[1]
	var args []Node
	hasArgs := false
	switch strings.ToLower(name) {
	case "alpha":
		if a := p.ieAlpha(); a != nil {
			p.in.forget()
			return a
		}
	case "boolean", "if":
		args = []Node{p.expectNode(p.condition(false, true), "expected condition")}
		hasArgs = true
	}
	args = p.arguments(args, hasArgs)
	if !p.in.char(')') {
		p.in.restore("Could not parse call arguments or missing ')'")
		return nil
	}
	p.in.forget()
	return newCall(name, args, index+p.currentIndex, p.fileInfo)
}

func (p *parser) declarationCall() Node {
	index := p.in.i
	p.in.save()
	valid, ok := p.in.re(reDeclCall)
	if !ok {
		p.in.forget()
		return nil
	}
	valid = js.Slice16(valid, 0, js.Len16(valid)-1)
	if reAtKeywordWord.Test(valid) {
		p.in.restore()
		return nil
	}
	rule := p.ruleProperty()
	var value Node
	if rule != nil {
		value = nodeOrNil(p.value())
	}
	var args []Node
	if rule != nil && value != nil {
		args = []Node{newDeclaration(rule, "", value, "", "", p.in.i+p.currentIndex, p.fileInfo, true, nil)}
	}
	if !p.in.char(')') {
		p.in.restore("Could not parse call arguments or missing ')'")
		return nil
	}
	p.in.forget()
	return newCall(valid, args, index+p.currentIndex, p.fileInfo)
}

// arguments parses call arguments; prev holds arguments a custom parser already produced.
func (p *parser) arguments(prev []Node, hasPrev bool) []Node {
	argsComma := prev
	var argsSemi []Node
	semi := false
	p.in.save()
	for {
		if hasPrev {
			hasPrev = false
		} else {
			value := p.detachedRuleset()
			if value == nil {
				value = p.assignment()
			}
			if value == nil {
				value = nodeOrNil(p.expression())
			}
			if value == nil {
				break
			}
			if e, ok := value.(*Expression); ok && len(e.value) == 1 {
				value = e.value[0]
			}
			argsComma = append(argsComma, value)
		}
		if p.in.char(',') {
			continue
		}
		if p.in.char(';') || semi {
			semi = true
			var value Node
			if len(argsComma) >= 1 {
				value = newValue(argsComma, 0)
			}
			argsSemi = append(argsSemi, value)
			argsComma = nil
		}
	}
	p.in.forget()
	if semi {
		return argsSemi
	}
	return argsComma
}

func (p *parser) literal() Node {
	if n := p.entDimension(); n != nil {
		return n
	}
	if n := p.entColor(); n != nil {
		return n
	}
	if n := p.entQuoted(false); n != nil {
		return n
	}
	return p.entUnicodeDescriptor()
}

func (p *parser) assignment() Node {
	p.in.save()
	key, ok := p.in.re(reAssignKey)
	if !ok || key == "" {
		p.in.restore()
		return nil
	}
	if !p.in.char('=') {
		p.in.restore()
		return nil
	}
	if value := p.entity(); value != nil {
		p.in.forget()
		return newAssignment(key, value)
	}
	p.in.restore()
	return nil
}

func hasInterpolation(text string) bool {
	return reHasVariableInterp.Test(text) || reHasPropertyInterp.Test(text)
}

func (p *parser) entURL() Node {
	index := p.in.i
	p.in.autoComment = false
	if !p.in.strTok("url(") {
		p.in.autoComment = true
		return nil
	}
	var value Node = p.entQuoted(false)
	if value == nil {
		value = p.entVariable()
	}
	if value == nil {
		value = p.entProperty()
	}
	raw := ""
	if value == nil {
		raw, _ = p.in.re(reURLBody)
		if hasInterpolation(raw) {
			value = newQuoted("", raw, true, index+p.currentIndex, p.fileInfo)
		}
	}
	p.in.autoComment = true
	p.expectChar(')')
	if value == nil {
		value = newAnonymous(raw, index, nil, false, false, nil)
	}
	return newURL(value, index+p.currentIndex, p.fileInfo, false)
}

func isSpaceChar(c int) bool {
	return c >= 0 && reSpaceStart.MatchAt([]uint16{uint16(c)}, 0) != nil
}

func (p *parser) entVariable() Node {
	index := p.in.i
	p.in.save()
	if p.in.currentChar() == '@' {
		if name, ok := p.in.re(reVariableName); ok {
			p.warnNumericVariableName(name, index)
			p.warnDashOnlyVariableName(name, index)
			ch := p.in.currentChar()
			if (ch == '(' || ch == '[') && !isSpaceChar(p.in.prevChar()) {
				if r := p.variableCall(name); r != nil {
					p.in.forget()
					return r
				}
			}
			p.in.forget()
			return newVariable(name, index+p.currentIndex, p.fileInfo)
		}
	}
	p.in.restore()
	return nil
}

func (p *parser) entVariableCurly() Node {
	index := p.in.i
	if p.in.currentChar() != '@' {
		return nil
	}
	p.in.save()
	m := p.in.match(reVariableCurly)
	if m == nil {
		p.in.restore()
		return nil
	}
	name := *m[1]
	lookups := p.ruleLookups()
	if !p.in.char('}') {
		p.in.restore()
		return nil
	}
	p.in.forget()
	p.warnNumericVariableName(name, index)
	p.warnDashOnlyVariableName(name, index)
	if lookups == nil {
		return newVariable("@"+name, index+p.currentIndex, p.fileInfo)
	}
	call := newVariableCall("@"+name, index+p.currentIndex, p.fileInfo)
	return newNamespaceValue(call, lookups, index+p.currentIndex, p.fileInfo)
}

func (p *parser) entProperty() Node {
	index := p.in.i
	if p.in.currentChar() == '$' {
		if name, ok := p.in.re(rePropertyName); ok {
			return newProperty(name, index+p.currentIndex, p.fileInfo)
		}
	}
	return nil
}

func (p *parser) entColor() Node {
	p.in.save()
	if p.in.currentChar() == '#' {
		if m := p.in.match(reColorHex); m != nil && m[2] == nil {
			p.in.forget()
			return newColorHex(*m[1], *m[0])
		}
	}
	p.in.restore()
	return nil
}

func (p *parser) entColorKeyword() Node {
	p.in.save()
	auto := p.in.autoComment
	p.in.autoComment = false
	k, ok := p.in.re(reColorKeyword)
	p.in.autoComment = auto
	if !ok {
		p.in.forget()
		return nil
	}
	p.in.restore()
	if c := colorFromKeyword(k); c != nil {
		p.in.strTok(k)
		return c
	}
	return nil
}

func (p *parser) entDimension() Node {
	if p.in.peekNotNumeric() {
		return nil
	}
	if m := p.in.match(reDimension); m != nil {
		unit := ""
		if m[2] != nil {
			unit = *m[2]
		}
		return newDimensionFromString(*m[1], unit)
	}
	return nil
}

func (p *parser) entUnicodeDescriptor() Node {
	if m := p.in.match(reUnicodeDescriptor); m != nil {
		return newUnicodeDescriptor(*m[0])
	}
	return nil
}

func (p *parser) entJavascript() Node {
	index := p.in.i
	p.in.save()
	escape := p.in.char('~')
	if !p.in.char('`') {
		p.in.restore()
		return nil
	}
	if s, ok := p.in.re(reJavascript); ok {
		p.warn("Inline JavaScript evaluation (backtick expressions) is deprecated and will be removed in Less 5.x. Use Less functions or custom plugins instead.", index, "DEPRECATED", "js-eval")
		p.in.forget()
		return newJavaScript(js.Slice16(s, 0, js.Len16(s)-1), escape, index+p.currentIndex, p.fileInfo)
	}
	p.in.restore("invalid javascript definition")
	return nil
}

// variable is the name part of a variable definition (`@fink:`).
func (p *parser) variable() string {
	if p.in.currentChar() == '@' {
		if m := p.in.match(reVariableDef); m != nil {
			at := p.in.i - js.Len16(*m[0])
			p.warnNumericVariableName(*m[1], at)
			p.warnDashOnlyVariableName(*m[1], at)
			return *m[1]
		}
	}
	return ""
}

// variableCall parses `@detached();` or a lookup; parsedName is set when called from a value.
func (p *parser) variableCall(parsedName string) Node {
	i := p.in.i
	inValue := parsedName != ""
	name := parsedName
	p.in.save()
	var m []*string
	if !inValue && p.in.currentChar() == '@' {
		m = p.in.match(reVariableCall)
	}
	if inValue || m != nil {
		lookups := p.ruleLookups()
		if lookups == nil {
			missing := true
			if inValue {
				p.in.strTok("()")
			} else {
				missing = m[2] == nil || *m[2] != "()"
			}
			if missing {
				p.in.restore("Missing '[...]' lookup in variable call")
				return nil
			}
		}
		if !inValue {
			p.warnNumericVariableName(*m[1], i)
			p.warnDashOnlyVariableName(*m[1], i)
			name = *m[1]
		}
		call := newVariableCall(name, i, p.fileInfo)
		if !inValue && p.end() {
			p.in.forget()
			return call
		}
		p.in.forget()
		return newNamespaceValue(call, lookups, i, p.fileInfo)
	}
	p.in.restore()
	return nil
}

func (p *parser) extend(isRule bool) []Node {
	index := p.in.i
	tok := ":extend("
	if isRule {
		tok = "&:extend("
	}
	if !p.in.strTok(tok) {
		return nil
	}
	var list []Node
	for {
		var option string
		var elements []*Element
		first := true
		for {
			if m := p.in.match(reExtendAll); m != nil {
				option = *m[1]
				break
			}
			e := p.element()
			if e == nil {
				break
			}
			if !first && e.combinator.value != "" {
				p.warn("Targeting complex selectors can have unexpected behavior, and this behavior may change in the future.", index, "", "")
			}
			first = false
			elements = append(elements, e)
		}
		if elements == nil {
			p.error("Missing target selector for :extend().")
		}
		list = append(list, newExtend(newSelector(elementNodes(elements), nil, nil, 0, nil, nil), option, index+p.currentIndex, p.fileInfo))
		if !p.in.char(',') {
			break
		}
	}
	p.expectRe(reCloseParen, "")
	if isRule {
		p.expectRe(reSemicolon, "")
	}
	return list
}

func elementNodes(es []*Element) []Node {
	out := make([]Node, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

// mixinCall parses a mixin call; getLookup is nil when undefined in the original.
func (p *parser) mixinCall(inValue bool, getLookup *bool) Node {
	s := p.in.currentChar()
	if s != '.' && s != '#' {
		return nil
	}
	index := p.in.i
	important := false
	p.in.save()
	elements := p.mixinElements()
	if elements != nil {
		parensIndex := p.in.i
		parensWS := p.in.isWhitespace(-1)
		var args []mixinArg
		hasParens := false
		if p.in.char('(') {
			args, _ = p.mixinArgs(true)
			p.expectChar(')')
			hasParens = true
			if parensWS {
				p.warn("Whitespace between a mixin name and parentheses for a mixin call is deprecated", parensIndex, "DEPRECATED", "mixin-call-whitespace")
			}
		}
		var lookups []string
		if getLookup == nil || *getLookup {
			lookups = p.ruleLookups()
		}
		if getLookup != nil && *getLookup && lookups == nil {
			p.in.restore()
			return nil
		}
		if inValue && lookups == nil && !hasParens {
			p.in.restore()
			return nil
		}
		if !inValue && p.important() != "" {
			important = true
		}
		if inValue || p.end() {
			p.in.forget()
			mixin := newMixinCall(elements, args, index+p.currentIndex, p.fileInfo, lookups == nil && important)
			for _, el := range elements {
				p.warnDashOnlyMixinName(el.valueStr, el.index-p.currentIndex)
			}
			if lookups != nil {
				return newNamespaceValue(mixin, lookups, 0, nil)
			}
			if !hasParens {
				p.warn("Calling a mixin without parentheses is deprecated", parensIndex, "DEPRECATED", "mixin-call-no-parens")
			}
			return mixin
		}
	}
	p.in.restore()
	return nil
}

func (p *parser) mixinLookup() Node { return p.mixinCall(true, boolp(true)) }

func (p *parser) mixinElements() []*Element {
	var elements []*Element
	var c *Combinator
	for {
		elemIndex := p.in.i
		e, ok := p.in.re(reMixinElement)
		if !ok {
			break
		}
		elements = append(elements, newElementStr(c, e, false, elemIndex+p.currentIndex, p.fileInfo))
		c = nil
		if p.in.char('>') {
			c = newCombinator(">")
		}
	}
	return elements
}

// mixinArg is one entry of a mixin call's or definition's argument list.
type mixinArg struct {
	name     string
	value    Node
	expand   bool
	variadic bool
}

// mixinArgs returns the arguments and whether the list ends in a variadic parameter.
func (p *parser) mixinArgs(isCall bool) ([]mixinArg, bool) {
	var expressions []Node
	var argsSemi, argsComma []mixinArg
	semi, containsNamed, variadic := false, false, false
	name := ""
	hasSep := true
	p.in.save()
	for {
		var arg Node
		if isCall {
			arg = p.detachedRuleset()
			if arg == nil {
				arg = nodeOrNil(p.expression())
			}
		} else {
			p.in.commentStore = nil
			if p.in.strTok("...") {
				variadic = true
				if p.in.char(';') && !semi {
					semi = true
				}
				if semi {
					argsSemi = append(argsSemi, mixinArg{variadic: true})
				} else {
					argsComma = append(argsComma, mixinArg{variadic: true})
				}
				break
			}
			arg = p.entVariable()
			if arg == nil {
				arg = p.entProperty()
			}
			if arg == nil {
				arg = p.literal()
			}
			if arg == nil {
				arg = p.entKeyword()
			}
			if arg == nil {
				arg = p.mixinCall(true, nil)
			}
		}
		if arg == nil || !hasSep {
			break
		}
		nameLoop := ""
		expand := false
		if e, ok := arg.(*Expression); ok {
			e.throwAwayComments()
		}
		value := arg
		var val Node
		if isCall {
			if e, ok := arg.(*Expression); ok && len(e.value) == 1 {
				val = e.value[0]
			}
		} else {
			val = arg
		}
		valName := ""
		switch v := val.(type) {
		case *Variable:
			valName = v.name
		case *Property:
			valName = v.name
		default:
			val = nil
		}
		if val != nil {
			if p.in.char(':') {
				if len(expressions) > 0 {
					if semi {
						p.error("Cannot mix ; and , as delimiter types")
					}
					containsNamed = true
				}
				value = p.detachedRuleset()
				if value == nil {
					value = nodeOrNil(p.expression())
				}
				if value == nil {
					if isCall {
						p.error("could not understand value for named argument")
					} else {
						p.in.restore()
						return nil, variadic
					}
				}
				name = valName
				nameLoop = name
			} else if p.in.strTok("...") {
				if !isCall {
					variadic = true
					if p.in.char(';') && !semi {
						semi = true
					}
					a := mixinArg{name: argName(arg), variadic: true}
					if semi {
						argsSemi = append(argsSemi, a)
					} else {
						argsComma = append(argsComma, a)
					}
					break
				}
				expand = true
			} else if !isCall {
				name = valName
				nameLoop = name
				value = nil
			}
		}
		if value != nil {
			expressions = append(expressions, value)
		}
		argsComma = append(argsComma, mixinArg{name: nameLoop, value: value, expand: expand})
		if p.in.char(',') {
			hasSep = true
			continue
		}
		hasSep = p.in.char(';')
		if hasSep || semi {
			if containsNamed {
				p.error("Cannot mix ; and , as delimiter types")
			}
			semi = true
			if len(expressions) > 1 {
				value = newValue(expressions, 0)
			}
			argsSemi = append(argsSemi, mixinArg{name: name, value: value, expand: expand})
			name = ""
			expressions = nil
			containsNamed = false
		}
	}
	p.in.forget()
	if semi {
		return argsSemi, variadic
	}
	return argsComma, variadic
}

func argName(n Node) string {
	switch v := n.(type) {
	case *Variable:
		return v.name
	case *Property:
		return v.name
	}
	return ""
}

func (p *parser) mixinDefinition() Node {
	index := p.in.i
	if c := p.in.currentChar(); (c != '.' && c != '#') || p.in.peekRe(reMixinDefPeek) {
		return nil
	}
	p.in.save()
	m := p.in.match(reMixinDefName)
	if m == nil {
		p.in.restore()
		return nil
	}
	name := *m[1]
	params, variadic := p.mixinArgs(false)
	if !p.in.char(')') {
		p.in.restore("Missing closing ')'")
		return nil
	}
	p.in.commentStore = nil
	var cond Node
	if p.in.strTok("when") {
		cond = p.expectNode(p.conditions(), "expected condition")
	}
	if rules := p.block(); rules != nil {
		p.in.forget()
		p.warnDashOnlyMixinName(name, index)
		return newMixinDefinition(name, params, rules, cond, variadic, nil, nil)
	}
	p.in.restore()
	return nil
}

// ruleLookups returns nil when there is no lookup; an entry may be the empty string.
func (p *parser) ruleLookups() []string {
	if p.in.currentChar() != '[' {
		return nil
	}
	var lookups []string
	for {
		p.in.save()
		rule, ok := p.lookupValue()
		if !ok {
			p.in.restore()
			break
		}
		lookups = append(lookups, rule)
		p.in.forget()
	}
	return lookups
}

func (p *parser) lookupValue() (string, bool) {
	p.in.save()
	if !p.in.char('[') {
		p.in.restore()
		return "", false
	}
	name, ok := p.in.re(reLookupName)
	if !p.in.char(']') {
		p.in.restore()
		return "", false
	}
	if ok {
		p.in.forget()
		return name, true
	}
	p.in.restore()
	return "", false
}

func (p *parser) entity() Node {
	if n := p.comment(); n != nil {
		return n
	}
	if n := p.literal(); n != nil {
		return n
	}
	if n := p.entVariable(); n != nil {
		return n
	}
	if n := p.entURL(); n != nil {
		return n
	}
	if n := p.entProperty(); n != nil {
		return n
	}
	if n := p.entCall(); n != nil {
		return n
	}
	if n := p.entKeyword(); n != nil {
		return n
	}
	if n := p.mixinCall(true, nil); n != nil {
		return n
	}
	return p.entJavascript()
}

func (p *parser) end() bool { return p.in.char(';') || p.in.peekStr("}") }

func (p *parser) ieAlpha() Node {
	if _, ok := p.in.re(reOpacity); !ok {
		return nil
	}
	value, ok := p.in.re(reDigits)
	if !ok {
		v := p.expectNode(p.entVariable(), "Could not parse alpha")
		name := ""
		if vv, isVar := v.(*Variable); isVar {
			name = vv.name
		} else {
			typeError("Cannot read properties of undefined (reading 'slice')")
		}
		value = "@{" + js.Slice16(name, 1, js.Len16(name)) + "}"
	}
	p.expectChar(')')
	return newQuoted("", "alpha(opacity="+value+")", false, 0, nil)
}

func (p *parser) element() *Element {
	index := p.in.i
	c := p.combinator()
	var e Node
	str := ""
	if s, ok := p.in.re(reElementPercent); ok {
		str = s
	} else if s, ok := p.in.re(reElementName); ok {
		str = s
	} else if p.in.char('*') {
		str = "*"
	} else if p.in.char('&') {
		str = "&"
	} else if a := p.attribute(); a != nil {
		e = a
	} else if s, ok := p.in.re(reElementParen); ok {
		str = s
	} else if s, ok := p.in.re(reElementInterpPrefix); ok {
		str = s
	} else if v := p.entVariableCurly(); v != nil {
		e = v
	}
	if e == nil && str == "" {
		p.in.save()
		if p.in.char('(') {
			if v := p.selector(false); v != nil {
				var sels []Node
				for p.in.char(',') {
					sels = append(sels, nodeOrNil(v), newAnonymous(",", 0, nil, false, false, nil))
					v = p.selector(false)
				}
				sels = append(sels, nodeOrNil(v))
				if p.in.char(')') {
					if len(sels) > 1 {
						e = newParen(newSelector(sels, nil, nil, 0, nil, nil))
					} else {
						e = newParen(v)
					}
					p.in.forget()
				} else {
					p.in.restore("Missing closing ')'")
				}
			} else {
				p.in.restore("Missing closing ')'")
			}
		} else {
			p.in.forget()
		}
	}
	if str != "" {
		return newElementStr(c, str, false, index+p.currentIndex, p.fileInfo)
	}
	if e != nil {
		_, isVar := e.(*Variable)
		return newElementNode(c, e, isVar, index+p.currentIndex, p.fileInfo)
	}
	return nil
}

func (p *parser) combinator() *Combinator {
	c := p.in.currentChar()
	if c == '/' {
		p.in.save()
		if s, ok := p.in.re(reSlashedCombinator); ok {
			p.in.forget()
			return newCombinator(s)
		}
		p.in.restore()
	}
	if c == '>' || c == '+' || c == '~' || c == '|' || c == '^' {
		p.in.i++
		s := string(rune(c))
		if c == '^' && p.in.currentChar() == '^' {
			s = "^^"
			p.in.i++
		}
		for p.in.isWhitespace(0) {
			p.in.i++
		}
		return newCombinator(s)
	} else if p.in.isWhitespace(-1) {
		return newCombinator(" ")
	}
	return newCombinator("")
}

func (p *parser) selector(isLess bool) *Selector {
	index := p.in.i
	var elements []Node
	var allExtends []Node
	var condition Node
	c := -1
	var extendList []Node
	when := false
	var e *Element
	for {
		matched := false
		if isLess {
			extendList = p.extend(false)
			matched = extendList != nil
		}
		if !matched && isLess {
			when = p.in.strTok("when")
			matched = when
		}
		if !matched {
			e = p.element()
			matched = e != nil
		}
		if !matched {
			break
		}
		switch {
		case when:
			condition = p.expectNode(p.conditions(), "expected condition")
		case condition != nil:
			p.error("CSS guard can only be used at the end of selector")
		case extendList != nil:
			allExtends = append(allExtends, extendList...)
		default:
			if allExtends != nil {
				p.error("Extend can only be used at the end of selector")
			}
			c = p.in.currentChar()
			elements = append(elements, e)
			e = nil
		}
		if c == '{' || c == '}' || c == ';' || c == ',' || c == ')' {
			break
		}
	}
	if elements != nil {
		return newSelector(elements, allExtends, condition, index+p.currentIndex, p.fileInfo, nil)
	}
	if allExtends != nil {
		p.error("Extend must be used to extend a selector, it cannot be used on its own")
	}
	return nil
}

func (p *parser) selectors() []*Selector {
	var sels []*Selector
	for {
		s := p.selector(true)
		if s == nil {
			break
		}
		sels = append(sels, s)
		p.in.commentStore = nil
		if s.condition != nil && len(sels) > 1 {
			p.error("Guards are only currently allowed on a single selector.")
		}
		if !p.in.char(',') {
			break
		}
		if s.condition != nil {
			p.error("Guards are only currently allowed on a single selector.")
		}
		p.in.commentStore = nil
	}
	return sels
}

func (p *parser) attribute() Node {
	if !p.in.char('[') {
		return nil
	}
	var key Node = p.entVariableCurly()
	keyStr := ""
	if key == nil {
		keyStr = p.expectRe(reAttributeKey, "")
	}
	var val Node
	valStr := ""
	op, hasOp := p.in.re(reAttributeOp)
	cif := ""
	if hasOp {
		val = p.entQuoted(false)
		if val == nil {
			if s, ok := p.in.re(reAttributePercent); ok {
				valStr = s
			} else if s, ok := p.in.re(reWord); ok {
				valStr = s
			} else {
				val = p.entVariableCurly()
			}
		}
		if val != nil || valStr != "" {
			cif, _ = p.in.re(reAttributeFlag)
		}
	}
	p.expectChar(']')
	return newAttribute(key, keyStr, op, val, valStr, cif)
}

func (p *parser) block() []Node {
	if p.in.char('{') {
		content := p.primary()
		if p.in.char('}') {
			return content
		}
	}
	return nil
}

func (p *parser) blockRuleset() *Ruleset {
	if b := p.block(); b != nil {
		return newRuleset(nil, b, false, nil)
	}
	return nil
}

func (p *parser) detachedRuleset() Node {
	p.in.save()
	var params []mixinArg
	hasParams, variadic := false, false
	if _, ok := p.in.re(reDetachedArgs); ok {
		params, variadic = p.mixinArgs(false)
		hasParams = true
		if !p.in.char(')') {
			p.in.restore()
			return nil
		}
	}
	if br := p.blockRuleset(); br != nil {
		p.in.forget()
		if hasParams {
			return newMixinDefinition("", params, br.rules, nil, variadic, nil, nil)
		}
		return newDetachedRuleset(br, nil)
	}
	p.in.restore()
	return nil
}

func (p *parser) ruleset() Node {
	p.in.save()
	var dbg *debugInfo
	if p.context("dumpLineNumbers").dumpLineNumbers != "" {
		dbg = p.getDebugInfo(p.in.i)
	}
	sels := p.selectors()
	if sels != nil {
		if rules := p.block(); rules != nil {
			p.in.forget()
			rs := newRuleset(sels, rules, p.context("strictImports").strictImports, nil)
			rs.debugInfo = dbg
			return rs
		}
	}
	p.in.restore()
	return nil
}

func (p *parser) declaration() Node {
	index := p.in.i
	c := p.in.currentChar()
	if c == '.' || c == '#' || c == '&' || c == ':' {
		return nil
	}
	p.in.save()
	nameStr := p.variable()
	var nameNodes []Node
	if nameStr == "" {
		nameNodes = p.ruleProperty()
	}
	if nameStr == "" && nameNodes == nil {
		p.in.restore()
		return nil
	}
	isVariable := nameStr != ""
	var value Node
	hasDR := false
	important, merge := "", ""
	if isVariable {
		if value = p.detachedRuleset(); value != nil {
			hasDR = true
		}
	}
	p.in.commentStore = nil
	if value == nil {
		if !isVariable && len(nameNodes) > 1 {
			last := nameNodes[len(nameNodes)-1]
			nameNodes = nameNodes[:len(nameNodes)-1]
			if k, ok := last.(*Keyword); ok {
				merge = k.value
			}
		}
		custom := false
		if !isVariable {
			if k, ok := nameNodes[0].(*Keyword); ok && strings.HasPrefix(k.value, "--") {
				custom = true
			}
		}
		if custom {
			if p.in.char(';') {
				value = newAnonymous("", 0, nil, false, false, nil)
			} else {
				value = p.permissiveValue(reUntilSemiBrace, false)
			}
		} else {
			value = p.anonymousValue()
		}
		if value != nil {
			p.in.forget()
			return newDeclaration(nameNodes, nameStr, value, "", merge, index+p.currentIndex, p.fileInfo, false, nil)
		}
		value = nodeOrNil(p.value())
		if value != nil {
			important = p.important()
		} else if isVariable {
			value = p.permissiveValue(nil, false)
		}
	}
	if value != nil && (p.end() || hasDR) {
		p.in.forget()
		return newDeclaration(nameNodes, nameStr, value, important, merge, index+p.currentIndex, p.fileInfo, false, nil)
	}
	p.in.restore()
	return nil
}

func (p *parser) anonymousValue() Node {
	index := p.in.i
	if m := p.in.match(reAnonymousValue); m != nil {
		return newAnonymous(*m[1], index+p.currentIndex, nil, false, false, nil)
	}
	return nil
}

// permissiveValue parses until one of the tokens; until is nil for the default ';'.
func (p *parser) permissiveValue(until *jsre.Regexp, deprecateVariables bool) Node {
	index := p.in.i
	var result []Node
	testCurrentChar := func() bool {
		ch := p.in.charAt(p.in.i)
		if until == nil {
			return ch == ";"
		}
		return until.Test(ch)
	}
	if testCurrentChar() {
		return nil
	}
	var value []Node
	for {
		e := p.comment()
		if e != nil {
			value = append(value, e)
			continue
		}
		if deprecateVariables {
			e = p.entVariableCurly()
			if e == nil {
				varIndex := p.in.i
				e = p.entity()
				if isBareVariableReference(e) {
					p.warnBareAtRuleVariable(varIndex)
				}
			}
		} else {
			e = p.entity()
		}
		if e != nil {
			value = append(value, e)
		}
		if p.in.peekStr(",") {
			value = append(value, newAnonymous(",", p.in.i, nil, false, false, nil))
			p.in.char(',')
		}
		if e == nil {
			break
		}
	}
	done := testCurrentChar()
	if len(value) > 0 {
		expr := newExpression(value, false)
		if done {
			return expr
		}
		result = append(result, expr)
		if p.in.prevChar() == ' ' {
			result = append(result, newAnonymous(" ", index, nil, false, false, nil))
		}
	}
	p.in.save()
	groups, stop, bareVar := p.in.parseUntil(until, deprecateVariables)
	if groups == nil && stop == "" {
		p.in.restore()
		return nil
	}
	if stop != "" {
		p.error("Expected '"+stop+"'", "Parse")
	}
	if len(groups) == 1 && groups[0].quote == "" && groups[0].text == " " {
		p.in.forget()
		return newAnonymous("", index, nil, false, false, nil)
	}
	if deprecateVariables && bareVar >= 0 {
		p.warnBareAtRuleVariable(bareVar)
	}
	for i, g := range groups {
		if g.quote != "" {
			result = append(result, newQuoted(g.quote, g.text, true, index, p.fileInfo))
			continue
		}
		item := g.text
		if i == len(groups)-1 {
			item = js.Trim(item)
		}
		q := newQuoted("'", item, true, index, p.fileInfo)
		bareOnly := rePropertyInterpAll.Replace(reVariableInterpAll.Replace(item, ""), "")
		if !deprecateVariables && reBareVariable.Test(bareOnly) {
			p.warn("@variable in unknown values will not be evaluated as variables in the future. Use @{variable}", index, "DEPRECATED", "variable-in-unknown-value")
		}
		if reBareProperty.Test(bareOnly) {
			p.warn("$property in unknown values will not be evaluated as property references in the future. Use ${property}", index, "DEPRECATED", "property-in-unknown-value")
		}
		q.variableRegex = reLookupVariableRegex
		q.propRegex = reLookupPropRegex
		result = append(result, q)
	}
	p.in.forget()
	return newExpression(result, true)
}

var (
	reLookupVariableRegex = jsre.MustCompile(`@(`+variableWithLookups+`)|@\{(`+variableWithLookups+`)\}`, "g")
	reLookupPropRegex     = jsre.MustCompile(`\$([\w-]+)|\$\{([\w-]+)\}`, "g")
)

// importOptions are the parenthesised @import options, in source order.
type importOptions struct {
	less, multiple, inline, reference, optional *bool
	isPlugin                                    bool
	pluginArgs                                  string
}

func (p *parser) importRule() Node {
	index := p.in.i
	if _, ok := p.in.re(reImport); !ok {
		return nil
	}
	opts := p.importOptions()
	if opts == nil {
		opts = &importOptions{}
	}
	path := p.entQuoted(false)
	if path == nil {
		path = p.entURL()
	}
	if path == nil {
		p.in.i = index
		p.error("malformed import statement")
	}
	features := p.mediaFeatures(true)
	if !p.in.char(';') {
		p.in.i = index
		p.error("missing semi-colon or unrecognised media features on import")
	}
	var feat Node
	if features != nil {
		feat = newValue(features, 0)
	}
	return newImport(path, feat, opts, index+p.currentIndex, p.fileInfo)
}

func (p *parser) importOptions() *importOptions {
	if !p.in.char('(') {
		return nil
	}
	o := &importOptions{}
	for {
		m := p.in.match(reImportOption)
		if m == nil {
			break
		}
		switch *m[1] {
		case "css":
			o.less = boolp(false)
		case "less":
			o.less = boolp(true)
		case "once":
			o.multiple = boolp(false)
		case "multiple":
			o.multiple = boolp(true)
		case "inline":
			o.inline = boolp(true)
		case "reference":
			o.reference = boolp(true)
		case "optional":
			o.optional = boolp(true)
		}
		if !p.in.char(',') {
			break
		}
	}
	p.expectChar(')')
	return o
}

func (p *parser) cssKeyword() Node {
	if k, ok := p.in.re(reCSSKeyword); ok {
		if c := colorFromKeyword(k); c != nil {
			return c
		}
		return newKeyword(k)
	}
	return nil
}

func (p *parser) mediaFeature(queryInParens bool) Node {
	var nodes []Node
	spacing := false
	p.in.save()
	for {
		p.in.save()
		if _, ok := p.in.re(reCSSKeywordParen); ok {
			spacing = true
		}
		p.in.restore()
		e := p.declarationCall()
		if e == nil {
			e = p.cssKeyword()
		}
		if e == nil {
			e = p.entKeyword()
		}
		if e == nil {
			e = p.entVariableCurly()
		}
		if e == nil {
			varIndex := p.in.i
			if bare := p.entVariable(); bare != nil {
				p.warnBareAtRuleVariable(varIndex)
				e = bare
			} else {
				e = p.mixinLookup()
			}
		}
		if e != nil {
			nodes = append(nodes, e)
			if e.Type() == "Variable" {
				spacing = true
			} else if k, ok := e.(*Keyword); ok && reMediaLogicWord.Test(k.value) {
				spacing = true
			}
		} else if p.in.char('(') {
			closed := false
			prop := p.property()
			var cond *Condition
			var rangeP *Condition
			p.in.save()
			if _, ok := p.in.re(reQueryRange); prop == "" && queryInParens && ok {
				p.in.restore()
				c := p.condition(false, false)
				cond, _ = c.(*Condition)
				p.in.save()
				if cond == nil {
					typeError("Cannot read properties of undefined (reading 'rvalue')")
				}
				rp := p.atomicCondition(false, cond.rvalue, false)
				rangeP, _ = rp.(*Condition)
				if rangeP == nil {
					p.in.restore()
				}
			} else {
				p.in.restore()
				p.in.save()
				e = nodeOrNil(p.value())
				if e != nil && p.in.char(')') {
					closed = true
					p.in.forget()
				} else {
					p.in.restore()
					e = p.mediaFeature(queryInParens)
				}
			}
			if !closed && p.in.char(')') {
				closed = true
			}
			if !closed {
				p.error("Missing closing ')'", "Parse")
			}
			hasP := prop != "" || cond != nil
			switch {
			case hasP && e == nil:
				var q *QueryInParens
				if rangeP != nil {
					q = newQueryInParens(cond.op, cond.lvalue, cond.rvalue, rangeP.op, rangeP.rvalue, cond.index)
				} else if cond != nil {
					q = newQueryInParens(cond.op, cond.lvalue, cond.rvalue, "", nil, cond.index)
				} else {
					q = newQueryInParens("", nil, nil, "", nil, 0)
				}
				paren := newParen(q)
				paren.noSpacing = !spacing
				nodes = append(nodes, paren)
				e = cond
			case hasP && e != nil:
				paren := newParen(newDeclaration(nil, prop, e, "", "", p.in.i+p.currentIndex, p.fileInfo, true, nil))
				paren.noSpacing = !spacing
				nodes = append(nodes, paren)
				spacing = false
			case e != nil:
				paren := newParen(e)
				paren.noSpacing = !spacing
				nodes = append(nodes, paren)
				spacing = false
			default:
				p.error("badly formed media feature definition")
			}
		}
		if e == nil {
			break
		}
	}
	p.in.forget()
	if len(nodes) > 0 {
		expr := newExpression(nodes, false)
		if len(nodes) == 2 && (nodes[0].Type() == "Keyword" || nodes[0].Type() == "Variable") {
			if par, ok := nodes[1].(*Paren); ok && par.noSpacing {
				expr.noSpacing = true
			}
		}
		return expr
	}
	return nil
}

func (p *parser) mediaFeatures(queryInParens bool) []Node {
	var features []Node
	for {
		e := p.mediaFeature(queryInParens)
		if e == nil {
			e = p.entVariableCurly()
			if e == nil {
				varIndex := p.in.i
				if bare := p.entVariable(); bare != nil {
					p.warnBareAtRuleVariable(varIndex)
					e = bare
				} else {
					e = p.mixinLookup()
				}
			}
			if e == nil {
				break
			}
		}
		features = append(features, e)
		if !p.in.char(',') {
			break
		}
	}
	return features
}

func (p *parser) nestableAtRule() Node {
	index := p.in.i
	var dbg *debugInfo
	if p.context("dumpLineNumbers").dumpLineNumbers != "" {
		dbg = p.getDebugInfo(index)
	}
	p.in.save()
	if p.in.peekCharIs('@') {
		isMedia := p.in.strTok("@media")
		if isMedia || p.in.strTok("@container") {
			features := p.mediaFeatures(true)
			rules := p.block()
			if rules == nil {
				p.error("media definitions require block statements after any features")
			}
			p.in.forget()
			var n Node
			if isMedia {
				m := newMedia(rules, features, index+p.currentIndex, p.fileInfo, nil)
				m.debugInfo = dbg
				n = m
			} else {
				c := newContainer(rules, features, index+p.currentIndex, p.fileInfo, nil)
				c.debugInfo = dbg
				n = c
			}
			return n
		}
	}
	p.in.restore()
	return nil
}

func (p *parser) plugin() Node {
	index := p.in.i
	re := rePlugin
	if p.disablePlugin {
		re = rePluginOff
	}
	if _, ok := p.in.re(re); !ok {
		return nil
	}
	if p.disablePlugin {
		p.error("@plugin statements are not allowed when disablePluginRule is set to true")
	}
	p.warn("The @plugin directive is deprecated and will be replaced in Less 5.x. Use --plugin CLI option or the programmatic plugin API instead.", index, "DEPRECATED", "at-plugin")
	opts := &importOptions{isPlugin: true}
	if args, ok := p.pluginArgs(); ok {
		opts.pluginArgs = args
	}
	path := p.entQuoted(false)
	if path == nil {
		path = p.entURL()
	}
	if path == nil {
		p.in.i = index
		p.error("malformed @plugin statement")
	}
	if !p.in.char(';') {
		p.in.i = index
		p.error("missing semi-colon on @plugin")
	}
	return newImport(path, nil, opts, index+p.currentIndex, p.fileInfo)
}

func (p *parser) pluginArgs() (string, bool) {
	p.in.save()
	if !p.in.char('(') {
		p.in.restore()
		return "", false
	}
	m := p.in.match(rePluginArgs)
	if m == nil {
		typeError("Cannot read properties of null (reading '1')")
	}
	p.in.forget()
	return js.Trim(*m[1]), true
}

func (p *parser) atRuleEntity() Node {
	if curly := p.entVariableCurly(); curly != nil {
		return curly
	}
	index := p.in.i
	e := p.entity()
	if isBareVariableReference(e) {
		p.warnBareAtRuleVariable(index)
	}
	return e
}

func (p *parser) atruleUnknown(name string) (Node, bool) {
	value := p.permissiveValue(reUntilBraceSemi, true)
	hasBlock := p.in.currentChar() == '{'
	if value == nil {
		if !hasBlock && p.in.currentChar() != ';' {
			p.error(name + " rule is missing block or ending semi-colon")
		}
	} else if a, ok := value.(*Anonymous); ok && a.valueNode == nil && a.value == "" {
		value = nil
	}
	return value, hasBlock
}

// atruleBlock returns the rules, the value (possibly replaced) and whether it became a keyword list.
func (p *parser) atruleBlock(value Node, isRooted bool) (*Ruleset, Node, []Node) {
	rules := p.blockRuleset()
	p.in.save()
	if rules == nil && !isRooted {
		value = p.atRuleEntity()
		rules = p.blockRuleset()
	}
	if rules == nil && !isRooted {
		p.in.restore()
		var list []Node
		value = p.atRuleEntity()
		for p.in.char(',') {
			list = append(list, value)
			value = p.atRuleEntity()
		}
		if value != nil && len(list) > 0 {
			return nil, value, append(list, value)
		}
		rules = p.blockRuleset()
	} else {
		p.in.forget()
	}
	return rules, value, nil
}

func (p *parser) atrule() Node {
	index := p.in.i
	if p.in.currentChar() != '@' {
		return nil
	}
	if v := p.importRule(); v != nil {
		return v
	}
	if v := p.plugin(); v != nil {
		return v
	}
	if v := p.nestableAtRule(); v != nil {
		return v
	}
	p.in.save()
	name, ok := p.in.re(reAtRuleName)
	if !ok {
		return nil
	}
	nonVendor := name
	if js.Slice16(name, 1, 2) == "-" {
		if k := js.IndexOf16(name, "-", 2); k > 0 {
			nonVendor = "@" + js.Slice16(name, k+1, js.Len16(name))
		}
	}
	hasIdentifier, hasExpression, hasUnknown := false, false, false
	hasBlock, isRooted := true, true
	switch nonVendor {
	case "@charset":
		hasIdentifier, hasBlock = true, false
	case "@namespace":
		hasExpression, hasBlock = true, false
	case "@keyframes", "@counter-style":
		hasIdentifier = true
	case "@document", "@supports":
		hasUnknown, isRooted = true, false
	case "@starting-style", "@layer":
		isRooted = false
	default:
		hasUnknown = true
	}
	p.in.commentStore = nil
	var value Node
	var keywordList []Node
	var rules *Ruleset
	switch {
	case hasIdentifier:
		value = p.atRuleEntity()
		if value == nil {
			p.error("expected " + name + " identifier")
		}
		if nonVendor == "@charset" {
			p.warnDynamicCharset(value, index)
		}
	case hasExpression:
		prefix := p.entVariableCurly()
		if prefix == nil && p.in.peekRe(reVariablePeek) {
			prefixIndex := p.in.i
			if prefix = p.entVariable(); prefix != nil {
				p.warnBareAtRuleVariable(prefixIndex)
			}
		}
		expr := p.expression()
		if expr != nil {
			value = expr
		}
		if prefix != nil {
			if expr != nil {
				value = newExpression(append([]Node{prefix}, expr.value...), false)
			} else {
				value = prefix
			}
		}
		if value == nil {
			p.error("expected " + name + " expression")
		}
	case hasUnknown:
		value, hasBlock = p.atruleUnknown(name)
	}
	if hasBlock {
		rules, value, keywordList = p.atruleBlock(value, isRooted)
		if rules == nil && !hasUnknown {
			p.in.restore()
			name, _ = p.in.re(reAtRuleName)
			value, hasBlock = p.atruleUnknown(name)
			if hasBlock {
				rules, value, keywordList = p.atruleBlock(value, isRooted)
			}
		}
	}
	if rules != nil || keywordList != nil || (!hasBlock && value != nil && p.in.char(';')) {
		p.in.forget()
		var dbg *debugInfo
		if p.context("dumpLineNumbers").dumpLineNumbers != "" {
			dbg = p.getDebugInfo(index)
		}
		var ruleList []Node
		if rules != nil {
			ruleList = []Node{rules}
		}
		return newAtRule(name, value, keywordList, ruleList, index+p.currentIndex, p.fileInfo, dbg, isRooted, nil)
	}
	p.in.restore("at-rule options not recognised")
	return nil
}

func (p *parser) value() *Value {
	index := p.in.i
	var exprs []Node
	for {
		e := p.expression()
		if e == nil {
			break
		}
		exprs = append(exprs, e)
		if !p.in.char(',') {
			break
		}
	}
	if len(exprs) > 0 {
		return newValue(exprs, index+p.currentIndex)
	}
	return nil
}

func (p *parser) important() string {
	if p.in.currentChar() == '!' {
		s, _ := p.in.re(reImportant)
		return s
	}
	return ""
}

func (p *parser) sub() Node {
	p.in.save()
	if p.in.char('(') {
		a := p.addition()
		if a != nil && p.in.char(')') {
			p.in.forget()
			e := newExpression([]Node{a}, false)
			e.parens = true
			return e
		}
		p.in.restore("Expected ')'")
		return nil
	}
	p.in.restore()
	return nil
}

func (p *parser) colorOperand() Node {
	p.in.save()
	if m := p.in.match(reColorOperand); m != nil {
		p.in.forget()
		return newKeyword(*m[1])
	}
	p.in.restore()
	return nil
}

func setParensInOp(n Node) {
	if po, ok := n.(interface{ setParensInOp() }); ok {
		po.setParensInOp()
	}
}

func (p *parser) multiplication() Node {
	m := p.operand()
	if m == nil {
		return nil
	}
	var operation Node
	isSpaced := p.in.isWhitespace(-1)
	for {
		if p.in.peekRe(reCommentPeek) {
			break
		}
		p.in.save()
		op := ""
		if p.in.char('/') {
			op = "/"
		} else if p.in.char('*') {
			op = "*"
		} else {
			index := p.in.i
			if p.in.strTok("./") {
				op = "./"
				p.warn("./ operator is deprecated", index, "DEPRECATED", "dot-slash-operator")
			}
		}
		if op == "" {
			p.in.forget()
			break
		}
		a := p.operand()
		if a == nil {
			p.in.restore()
			break
		}
		p.in.forget()
		setParensInOp(m)
		setParensInOp(a)
		left := operation
		if left == nil {
			left = m
		}
		operation = newOperation(op, []Node{left, a}, isSpaced)
		isSpaced = p.in.isWhitespace(-1)
	}
	if operation != nil {
		return operation
	}
	return m
}

func (p *parser) addition() Node {
	m := p.multiplication()
	if m == nil {
		return nil
	}
	var operation Node
	isSpaced := p.in.isWhitespace(-1)
	for {
		op, ok := p.in.re(reAddOp)
		if !ok && !isSpaced {
			if p.in.char('+') {
				op, ok = "+", true
			} else if p.in.char('-') {
				op, ok = "-", true
			}
		}
		if !ok {
			break
		}
		a := p.multiplication()
		if a == nil {
			break
		}
		setParensInOp(m)
		setParensInOp(a)
		left := operation
		if left == nil {
			left = m
		}
		operation = newOperation(op, []Node{left, a}, isSpaced)
		isSpaced = p.in.isWhitespace(-1)
	}
	if operation != nil {
		return operation
	}
	return m
}

func (p *parser) conditions() Node {
	index := p.in.i
	a := p.condition(true, false)
	if a == nil {
		return nil
	}
	var cond Node
	for {
		if !p.in.peekRe(reConditionsPeek) || !p.in.char(',') {
			break
		}
		b := p.condition(true, false)
		if b == nil {
			break
		}
		left := cond
		if left == nil {
			left = a
		}
		cond = newCondition("or", left, b, index+p.currentIndex, false)
	}
	if cond != nil {
		return cond
	}
	return a
}

func (p *parser) condition(needsParens, allowOperands bool) Node {
	result := p.conditionAnd(needsParens, allowOperands)
	if result == nil {
		return nil
	}
	if p.in.strTok("or") {
		next := p.condition(needsParens, allowOperands)
		if next == nil {
			return nil
		}
		result = newCondition("or", result, next, 0, false)
	}
	return result
}

func (p *parser) conditionAnd(needsParens, allowOperands bool) Node {
	result := p.negatedCondition(needsParens, allowOperands)
	if result == nil {
		result = p.parenthesisCondition(needsParens, allowOperands)
	}
	if result == nil && !needsParens {
		result = p.atomicCondition(needsParens, nil, false)
	}
	if result == nil {
		return nil
	}
	if allowOperands {
		if r := p.atomicCondition(needsParens, result, allowOperands); r != nil {
			result = r
		}
	}
	if p.in.strTok("and") {
		next := p.conditionAnd(needsParens, allowOperands)
		if next == nil {
			return nil
		}
		result = newCondition("and", result, next, 0, false)
	}
	return result
}

func (p *parser) negatedCondition(needsParens, allowOperands bool) Node {
	if !p.in.strTok("not") {
		return nil
	}
	if result := p.parenthesisCondition(needsParens, allowOperands); result != nil {
		if c, ok := result.(*Condition); ok {
			c.negate = !c.negate
		}
		return result
	}
	index := p.in.i
	a := p.entKeyword()
	if a == nil {
		a = p.entVariable()
	}
	if a == nil {
		a = p.entQuoted(false)
	}
	if a == nil {
		a = p.mixinLookup()
	}
	if a != nil {
		return newCondition("=", a, newKeyword("true"), index+p.currentIndex, true)
	}
	return nil
}

func (p *parser) parenthesisCondition(needsParens, allowOperands bool) Node {
	p.in.save()
	if !p.in.strTok("(") {
		p.in.restore()
		return nil
	}
	p.in.save()
	if body := p.condition(needsParens, allowOperands); body != nil {
		if p.in.char(')') {
			p.in.forget()
			p.in.forget()
			return body
		}
	}
	p.in.restore()
	body := p.atomicCondition(needsParens, nil, false)
	if body == nil {
		p.in.restore()
		return nil
	}
	if !p.in.char(')') {
		p.in.restore("expected ')' got '" + p.in.charAt(p.in.i) + "'")
		return nil
	}
	p.in.forget()
	return body
}

func (p *parser) atomicCondition(needsParens bool, preparsed Node, allowOperands bool) Node {
	index := p.in.i
	cond := func() Node {
		if allowOperands {
			if n := p.parenthesisCondition(needsParens, false); n != nil {
				return n
			}
		}
		if n := p.addition(); n != nil {
			return n
		}
		if n := p.entKeyword(); n != nil {
			return n
		}
		if n := p.entQuoted(false); n != nil {
			return n
		}
		return p.mixinLookup()
	}
	a := preparsed
	if a == nil {
		a = cond()
	}
	if a == nil {
		return nil
	}
	op := ""
	if p.in.char('>') {
		op = ">"
		if p.in.char('=') {
			op = ">="
		}
	} else if p.in.char('<') {
		op = "<"
		if p.in.char('=') {
			op = "<="
		}
	} else if p.in.char('=') {
		op = "="
		if p.in.char('>') {
			op = "=>"
		} else if p.in.char('<') {
			op = "=<"
		}
	}
	if op != "" {
		b := cond()
		if b == nil {
			p.error("expected expression")
		}
		return newCondition(op, a, b, index+p.currentIndex, false)
	}
	if preparsed == nil {
		return newCondition("=", a, newKeyword("true"), index+p.currentIndex, false)
	}
	return nil
}

func (p *parser) operand() Node {
	negate := false
	if p.in.peekRe(reNegatePeek) {
		negate = p.in.char('-')
	}
	o := p.sub()
	if o == nil {
		o = p.entDimension()
	}
	if o == nil {
		o = p.entColor()
	}
	if o == nil {
		o = p.entVariable()
	}
	if o == nil {
		o = p.entProperty()
	}
	if o == nil {
		o = p.entCall()
	}
	if o == nil {
		o = p.entQuoted(true)
	}
	if o == nil {
		o = p.entColorKeyword()
	}
	if o == nil {
		o = p.colorOperand()
	}
	if o == nil {
		o = p.mixinLookup()
	}
	if negate {
		if o == nil {
			typeError("Cannot set properties of undefined (setting 'parensInOp')")
		}
		setParensInOp(o)
		o = newNegative(o)
	}
	return o
}

func (p *parser) expression() *Expression {
	index := p.in.i
	var entities []Node
	for {
		e := p.comment()
		if e != nil {
			if c := e.(*Comment); !c.isLineComment {
				entities = append(entities, e)
				continue
			}
		}
		e = p.addition()
		if e == nil {
			e = p.entity()
		}
		if _, isComment := e.(*Comment); isComment {
			e = nil
		}
		if e == nil {
			break
		}
		entities = append(entities, e)
		if !p.in.peekRe(reCommentPeek) {
			if p.in.char('/') {
				entities = append(entities, newAnonymous("/", index+p.currentIndex, nil, false, false, nil))
			}
		}
	}
	if len(entities) > 0 {
		return newExpression(entities, false)
	}
	return nil
}

func (p *parser) property() string {
	if m := p.in.match(reProperty); m != nil {
		return *m[1]
	}
	return ""
}

func (p *parser) ruleProperty() []Node {
	p.in.save()
	if m := p.in.match(reSimpleProperty); m != nil {
		p.in.forget()
		return []Node{newKeyword(*m[1])}
	}
	var name []string
	var index []int
	match := func(r *jsre.Regexp) bool {
		i := p.in.i
		m := p.in.match(r)
		if m == nil {
			return false
		}
		index = append(index, i)
		name = append(name, *m[1])
		return true
	}
	match(reStar)
	for match(reRulePropertyParticle) {
	}
	if len(name) > 1 && match(reMergeColon) {
		p.in.forget()
		if name[0] == "" {
			name, index = name[1:], index[1:]
		}
		out := make([]Node, len(name))
		for k, s := range name {
			sigil := js.Slice16(s, 0, 1)
			if sigil != "@" && sigil != "$" {
				out[k] = newKeyword(s)
				continue
			}
			raw := js.Slice16(s, 2, js.Len16(s)-1)
			if sigil == "@" {
				base, _ := splitLookups(raw)
				p.warnNumericVariableName(base, index[k])
				p.warnDashOnlyVariableName(base, index[k])
				out[k] = resolveInterpolatedVariable(raw, index[k]+p.currentIndex, p.fileInfo)
			} else {
				out[k] = newProperty("$"+raw, index[k]+p.currentIndex, p.fileInfo)
			}
		}
		return out
	}
	p.in.restore()
	return nil
}

var reLookupSegment = jsre.MustCompile(`\[(`+lookupKey+`)\]`, "g")

// splitLookups splits `name[a][b]` into the name and its lookup keys (nil when there is no '[').
func splitLookups(raw string) (string, []string) {
	open := js.IndexOf16(raw, "[", 0)
	if open < 0 {
		return raw, nil
	}
	lookups := []string{}
	in := js.ToU16(raw)
	pos := 0
	for pos <= len(in) {
		caps := reLookupSegment.Search(in, pos)
		if caps == nil {
			break
		}
		lookups = append(lookups, js.FromU16(in[caps[2]:caps[3]]))
		pos = caps[1]
	}
	return js.Slice16(raw, 0, open), lookups
}

func resolveInterpolatedVariable(raw string, index int, fi *FileInfo) Node {
	name, lookups := splitLookups(raw)
	if lookups == nil {
		return newVariable("@"+name, index, fi)
	}
	return newNamespaceValue(newVariableCall("@"+name, index, fi), lookups, index, fi)
}
