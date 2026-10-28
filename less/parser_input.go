package less

import (
	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// The parser works on UTF-16 code units so that every index, slice and column matches the JavaScript original.

type storedComment struct {
	index         int
	text          string
	isLineComment bool
}

type inputState struct {
	curStart int
	i        int
}

type parserInput struct {
	input        []uint16
	i            int
	curStart     int // start of `current`, which is always a suffix of the input
	currentPos   int
	saveStack    []inputState
	furthest     int
	furthestMsg  string
	hasFurthest  bool
	commentStore []storedComment
	autoComment  bool
	finished     bool
}

type endState struct {
	isFinished         bool
	furthest           int
	furthestMessage    string
	furthestReachedEnd bool
	furthestChar       string
}

func newParserInput(s string) *parserInput {
	p := &parserInput{input: js.ToU16(s), autoComment: true}
	p.skipWhitespace(0)
	return p
}

// at returns the code unit at pos or -1 outside the input (charCodeAt gives NaN there).
func (p *parserInput) at(pos int) int {
	if pos < 0 || pos >= len(p.input) {
		return -1
	}
	return int(p.input[pos])
}

func (p *parserInput) str(a, b int) string {
	if a < 0 {
		a = 0
	}
	if b > len(p.input) {
		b = len(p.input)
	}
	if a >= b {
		return ""
	}
	return js.FromU16(p.input[a:b])
}

func (p *parserInput) charAt(pos int) string {
	if pos < 0 || pos >= len(p.input) {
		return ""
	}
	return js.FromU16(p.input[pos : pos+1])
}

func (p *parserInput) curLen() int { return len(p.input) - p.curStart }

// sliceCurrent mirrors current = current.slice(k) for any k, including negative ones.
func (p *parserInput) sliceCurrent(k int) {
	n := p.curLen()
	if k < 0 {
		k += n
		if k < 0 {
			k = 0
		}
	}
	if k > n {
		k = n
	}
	p.curStart += k
}

func (p *parserInput) indexOf(sub string, from int) int {
	u := js.ToU16(sub)
	for i := from; i+len(u) <= len(p.input); i++ {
		ok := true
		for k := range u {
			if p.input[i+k] != u[k] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func (p *parserInput) skipWhitespace(length int) bool {
	oldi := p.i
	curr := p.i - p.currentPos
	endIndex := p.i + p.curLen() - curr
	p.i += length
	mem := p.i
	for ; p.i < endIndex; p.i++ {
		c := p.at(p.i)
		if p.autoComment && c == '/' {
			next := p.at(p.i + 1)
			if next == '/' {
				idx := p.i
				nl := p.indexOf("\n", p.i+2)
				if nl < 0 {
					nl = endIndex
				}
				p.i = nl
				p.commentStore = append(p.commentStore, storedComment{index: idx, text: p.str(idx, p.i), isLineComment: true})
				continue
			} else if next == '*' {
				end := p.indexOf("*/", p.i+2)
				if end >= 0 {
					text := p.str(p.i, end+2)
					p.commentStore = append(p.commentStore, storedComment{index: p.i, text: text})
					p.i += end + 2 - p.i - 1
					continue
				}
			}
			break
		}
		if c != ' ' && c != '\n' && c != '\t' && c != '\r' {
			break
		}
	}
	p.sliceCurrent(length + p.i - mem + curr)
	p.currentPos = p.i
	if p.curLen() == 0 {
		p.finished = true
	}
	return oldi != p.i
}

func (p *parserInput) save() {
	p.currentPos = p.i
	p.saveStack = append(p.saveStack, inputState{curStart: p.curStart, i: p.i})
}

func (p *parserInput) restore(msg ...string) {
	m := ""
	if len(msg) > 0 {
		m = msg[0]
	}
	if p.i > p.furthest || (p.i == p.furthest && m != "" && !p.hasFurthest) {
		p.furthest = p.i
		p.furthestMsg = m
		p.hasFurthest = m != ""
	}
	if len(p.saveStack) == 0 {
		typeError("Cannot read properties of undefined (reading 'current')")
	}
	st := p.saveStack[len(p.saveStack)-1]
	p.saveStack = p.saveStack[:len(p.saveStack)-1]
	p.curStart = st.curStart
	p.i = st.i
	p.currentPos = st.i
}

// forget drops the last saved state; like Array.prototype.pop it does nothing on an empty stack.
func (p *parserInput) forget() {
	if len(p.saveStack) > 0 {
		p.saveStack = p.saveStack[:len(p.saveStack)-1]
	}
}

func (p *parserInput) isWhitespace(offset int) bool {
	c := p.at(p.i + offset)
	return c == ' ' || c == '\r' || c == '\t' || c == '\n'
}

// match runs an anchored pattern on `current` and returns the capture strings (nil for an unset group).
func (p *parserInput) match(re *jsre.Regexp) []*string {
	if p.i > p.currentPos {
		p.sliceCurrent(p.i - p.currentPos)
		p.currentPos = p.i
	}
	cur := p.input[p.curStart:]
	caps := re.MatchAt(cur, 0)
	if caps == nil {
		return nil
	}
	out := make([]*string, len(caps)/2)
	for g := range out {
		if caps[2*g] >= 0 {
			s := js.FromU16(cur[caps[2*g]:caps[2*g+1]])
			out[g] = &s
		}
	}
	p.skipWhitespace(caps[1] - caps[0])
	return out
}

// re is $re for a pattern without groups: the matched text, or "" and false.
func (p *parserInput) re(re *jsre.Regexp) (string, bool) {
	m := p.match(re)
	if m == nil {
		return "", false
	}
	return *m[0], true
}

func (p *parserInput) char(tok byte) bool {
	if p.at(p.i) != int(tok) {
		return false
	}
	p.skipWhitespace(1)
	return true
}

func (p *parserInput) peekCharIs(tok byte) bool { return p.at(p.i) == int(tok) }

func (p *parserInput) strTok(tok string) bool {
	u := js.ToU16(tok)
	for k := range u {
		if p.at(p.i+k) != int(u[k]) {
			return false
		}
	}
	p.skipWhitespace(len(u))
	return true
}

// quotedAt implements $quoted(loc) with an explicit location: it returns the quote char and the whole string.
func (p *parserInput) quotedAt(pos int) (string, string, bool) {
	start := p.at(pos)
	if start != '\'' && start != '"' {
		return "", "", false
	}
	for i := 1; i+pos < len(p.input); i++ {
		switch c := p.at(i + pos); {
		case c == '\\':
			i++
		case c == start:
			return string(rune(start)), p.str(pos, pos+i+1), true
		}
	}
	return "", "", false
}

// quoted implements $quoted() at the current position, consuming the string.
func (p *parserInput) quoted() (string, bool) {
	_, s, ok := p.quotedAt(p.i)
	if !ok {
		return "", false
	}
	p.skipWhitespace(js.Len16(s))
	return s, true
}

func (p *parserInput) peekStr(tok string) bool {
	u := js.ToU16(tok)
	for k := range u {
		if p.at(p.i+k) != int(u[k]) {
			return false
		}
	}
	return true
}

func (p *parserInput) peekRe(re *jsre.Regexp) bool {
	return re.MatchAt(p.input[p.curStart:], 0) != nil
}

func (p *parserInput) currentChar() int { return p.at(p.i) }

func (p *parserInput) prevChar() int { return p.at(p.i - 1) }

func (p *parserInput) peekNotNumeric() bool {
	c := p.at(p.i)
	if c < 0 {
		return false
	}
	return c > '9' || c < '+' || c == '/' || c == ','
}

func (p *parserInput) end() endState {
	msg := ""
	finished := p.i >= len(p.input)
	if p.i < p.furthest {
		msg = p.furthestMsg
		p.i = p.furthest
	}
	return endState{
		isFinished:         finished,
		furthest:           p.i,
		furthestMessage:    msg,
		furthestReachedEnd: p.i >= len(p.input)-1,
		furthestChar:       p.charAt(p.i),
	}
}

// untilGroup is one item of $parseUntil's result: raw text, or a quoted string with its quote character.
type untilGroup struct {
	quote string
	text  string
}

// parseUntil implements $parseUntil. It returns the groups, or the expected closing character (or the open
// quote) when the input is malformed; both are empty when nothing was found. bareVar is -1 when unset.
func (p *parserInput) parseUntil(tok *jsre.Regexp, detectBareVar bool) (groups []untilGroup, stop string, bareVar int) {
	bareVar = -1
	blockDepth, parenDepth := 0, 0
	var blockStack []int
	length := len(p.input)
	startPos := p.i
	lastPos := p.i
	i := p.i
	inComment := false
	testChar := func(pos int) bool {
		if tok == nil {
			return p.at(pos) == ';'
		}
		return tok.Test(p.charAt(pos))
	}
	for {
		c := p.at(i)
		if blockDepth == 0 && testChar(i) {
			if s := p.str(lastPos, i); s != "" {
				groups = append(groups, untilGroup{text: s})
			} else {
				groups = append(groups, untilGroup{text: " "})
			}
			p.skipWhitespace(i - startPos)
			return groups, "", bareVar
		}
		if inComment {
			if c == '*' && p.at(i+1) == '/' {
				i++
				blockDepth--
				inComment = false
			}
			i++
			continue
		}
		if detectBareVar && bareVar < 0 && c == '@' && parenDepth == 0 {
			if a := p.at(i + 1); a >= 0 && (a == '-' || a == '_' || (a >= '0' && a <= '9') || (a|0x20 >= 'a' && a|0x20 <= 'z')) {
				bareVar = i
			}
		}
		switch c {
		case '\\':
			i++
			groups = append(groups, untilGroup{text: p.str(lastPos, i+1)})
			lastPos = i + 1
		case '/':
			if p.at(i+1) == '*' {
				i++
				inComment = true
				blockDepth++
			}
		case '\'', '"':
			if q, s, ok := p.quotedAt(i); ok {
				groups = append(groups, untilGroup{text: p.str(lastPos, i)}, untilGroup{quote: q, text: s})
				i += js.Len16(s) - 1
				lastPos = i + 1
			} else {
				p.skipWhitespace(i - startPos)
				return nil, string(rune(c)), bareVar
			}
		case '{':
			blockStack = append(blockStack, '}')
			blockDepth++
		case '(':
			blockStack = append(blockStack, ')')
			blockDepth++
			parenDepth++
		case '[':
			blockStack = append(blockStack, ']')
			blockDepth++
		case '}', ')', ']':
			expected := -1
			if len(blockStack) > 0 {
				expected = blockStack[len(blockStack)-1]
				blockStack = blockStack[:len(blockStack)-1]
			}
			if c == expected {
				blockDepth--
				if c == ')' && parenDepth > 0 {
					parenDepth--
				}
			} else {
				p.skipWhitespace(i - startPos)
				if expected < 0 {
					return nil, "", bareVar
				}
				return nil, string(rune(expected)), bareVar
			}
		}
		i++
		if i > length {
			return nil, "", bareVar
		}
	}
}
