package less

import (
	"math"
	"strings"

	"github.com/less/less.js/internal/js"
)

// errorInfo is a thrown error before it is wrapped: a plain object like {type, message, index, filename} or a
// LessError. Index is a JavaScript number that may be undefined.
type errorInfo struct {
	Type     string
	Message  string
	Index    *int
	Filename string
	Call     *int
	isLess   bool
	// jsError marks a JavaScript Error (TypeError and the like), whose message is not an enumerable property
	jsError bool
	// lineColumnSet marks a plain error object with own line and column properties (Call.eval's wrapper)
	lineColumnSet bool
	// set on a wrapped LessError
	Line        *int
	lineSet     bool // false while LessError left line undefined (no file contents)
	Column      int
	CallLine    *int
	CallExtract *string
	Extract     [3]*string
}

func (e *errorInfo) Error() string { return e.Message }

// hasLineColumn is e.hasOwnProperty('line') && e.hasOwnProperty('column').
func (e *errorInfo) hasLineColumn() bool { return e.lineSet || e.lineColumnSet }

func intp(i int) *int { return &i }

// throwError panics with a plain error object, the way the nodes `throw { type, message, index, filename }`.
func throwError(typ, msg string, index int, fi *FileInfo) {
	e := &errorInfo{Type: typ, Message: msg, Index: intp(index)}
	if fi != nil {
		e.Filename = fi.Filename
	}
	panic(e)
}

// fileContents maps filenames to their parsed contents (imports.contents).
type fileContents interface {
	contentsOf(filename string) (string, bool)
}

// newLessError is new LessError(e, fileContentMap, currentFilename).
func newLessError(e *errorInfo, files fileContents, currentFilename string) *errorInfo {
	le := &errorInfo{isLess: true, Message: e.Message, Type: e.Type}
	if le.Type == "" {
		le.Type = "Syntax"
	}
	filename := e.Filename
	if filename == "" {
		filename = currentFilename
	}
	if files == nil || filename == "" {
		return le
	}
	input, _ := files.contentsOf(filename)
	line, col := getLocation(e.Index, input)
	le.Filename = filename
	le.Index = e.Index
	le.lineSet = true
	if line != nil {
		le.Line = intp(*line + 1)
	}
	le.Column = col
	var lines []string
	if input != "" {
		lines = js.Split16(input, "\n")
	}
	lineAt := func(i int) *string {
		if i < 0 || i >= len(lines) {
			return nil
		}
		return &lines[i]
	}
	if e.Call != nil {
		cl, _ := getLocation(e.Call, input)
		le.CallLine = intp(*cl + 1)
		le.CallExtract = lineAt(*cl)
	}
	if le.Line != nil {
		le.Extract = [3]*string{lineAt(*le.Line - 2), lineAt(*le.Line - 1), lineAt(*le.Line)}
	}
	return le
}

// getLocation returns the line (nil when index is undefined) and the column in UTF-16 units.
func getLocation(index *int, input string) (*int, int) {
	if index == nil {
		return nil, -1
	}
	u := js.ToU16(input)
	column := -1
	n := *index + 1
	for {
		n--
		if n < 0 || (n < len(u) && u[n] == '\n') {
			break
		}
		column++
	}
	end := *index
	if end > len(u) {
		end = len(u)
	}
	line := 0
	for i := 0; i < end; i++ {
		if u[i] == '\n' {
			line++
		}
	}
	return &line, column
}

type stylizer func(s, style string) string

var ansiStyles = map[string][2]string{
	"reset": {"0", "0"}, "bold": {"1", "22"}, "inverse": {"7", "27"}, "underline": {"4", "24"},
	"yellow": {"33", "39"}, "green": {"32", "39"}, "red": {"31", "39"}, "grey": {"90", "39"},
}

// ansiStylize is lessc-helper.js stylize.
func ansiStylize(s, style string) string {
	st := ansiStyles[style]
	return "\x1b[" + st[0] + "m" + s + "\x1b[" + st[1] + "m"
}

func plainStylize(s, _ string) string { return s }

// String is LessError.prototype.toString.
func (e *errorInfo) String(stylize stylizer) string {
	if stylize == nil {
		stylize = plainStylize
	}
	isWarning := strings.Contains(strings.ToLower(e.Type), "warning")
	typ := e.Type + "Error"
	color := "red"
	if isWarning {
		typ = e.Type
		color = "yellow"
	}
	message := ""
	errText := ""
	if e.Line != nil || !e.lineSet {
		var parts []string
		line := math.NaN()
		if e.Line != nil {
			line = float64(*e.Line)
		}
		if !isWarning && e.Extract[0] != nil {
			parts = append(parts, stylize(js.NumberToString(line-1)+" "+*e.Extract[0], "grey"))
		}
		if e.Extract[1] != nil {
			t := js.NumberToString(line) + " "
			if x := *e.Extract[1]; x != "" {
				t += js.Slice16(x, 0, e.Column) + stylize(stylize(stylize(js.Slice16(x, e.Column, e.Column+1), "bold")+
					js.Slice16(x, e.Column+1, js.Len16(x)), "red"), "inverse")
			}
			parts = append(parts, t)
		}
		if !isWarning && e.Extract[2] != nil {
			parts = append(parts, stylize(js.NumberToString(line+1)+" "+*e.Extract[2], "grey"))
		}
		errText = strings.Join(parts, "\n") + stylize("", "reset") + "\n"
	}
	message += stylize(typ+": "+e.Message, color)
	if e.Filename != "" {
		message += stylize(" in ", color) + e.Filename
	}
	if e.Line != nil && *e.Line != 0 {
		message += stylize(" on line "+itoa(*e.Line)+", column "+itoa(e.Column+1)+":", "grey")
	}
	message += "\n" + errText
	if e.CallLine != nil && *e.CallLine != 0 {
		ce := "undefined"
		if e.CallExtract != nil {
			ce = *e.CallExtract
		}
		message += stylize("from ", color) + e.Filename + "/n"
		message += stylize(itoa(*e.CallLine), "grey") + " " + ce + "/n"
	}
	return message
}
