// Package sourcemap is the part of the source-map 0.6.1 library that Less uses: a version 3 source map generator
// with base64 VLQ mappings, serialised exactly as JSON.stringify(generator.toJSON()) prints it.
package sourcemap

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/less/less.js/internal/js"
)

// Position is a line (1-based) and column (0-based) pair.
type Position struct {
	Line   int
	Column int
}

// Mapping is one argument of addMapping. Original and Source may be nil (null in JavaScript).
type Mapping struct {
	Generated Position
	Original  *Position
	Source    *string
	Name      *string
}

type entry struct {
	generatedLine, generatedColumn int
	originalLine, originalColumn   int
	hasOriginal                    bool
	source, name                   *string
}

// Generator is SourceMapGenerator.
type Generator struct {
	file            *string
	sources         arraySet
	names           arraySet
	mappings        []entry
	sorted          bool
	sourcesContents map[string]string
}

// NewGenerator returns a generator for the named output file (nil when there is no file name).
func NewGenerator(file *string) *Generator {
	return &Generator{file: file, sorted: true}
}

// InvalidMappingError is the Error addMapping throws for a mapping it rejects.
type InvalidMappingError struct{ Message string }

func (e *InvalidMappingError) Error() string { return e.Message }

// AddMapping is SourceMapGenerator.prototype.addMapping, including its validation.
func (g *Generator) AddMapping(m Mapping) error {
	if err := validate(m); err != nil {
		return err
	}
	if m.Source != nil && !g.sources.has(*m.Source) {
		g.sources.add(*m.Source)
	}
	if m.Name != nil && !g.names.has(*m.Name) {
		g.names.add(*m.Name)
	}
	e := entry{generatedLine: m.Generated.Line, generatedColumn: m.Generated.Column, source: m.Source, name: m.Name}
	if m.Original != nil {
		e.hasOriginal = true
		e.originalLine, e.originalColumn = m.Original.Line, m.Original.Column
	}
	if n := len(g.mappings); n > 0 && !generatedPositionAfter(&g.mappings[n-1], &e) {
		g.sorted = false
	}
	g.mappings = append(g.mappings, e)
	return nil
}

func validate(m Mapping) error {
	gen := m.Generated
	genOK := gen.Line > 0 && gen.Column >= 0
	if genOK && m.Original == nil && (m.Source == nil || *m.Source == "") && (m.Name == nil || *m.Name == "") {
		return nil
	}
	if genOK && m.Original != nil && m.Original.Line > 0 && m.Original.Column >= 0 && m.Source != nil && *m.Source != "" {
		return nil
	}
	var b strings.Builder
	b.WriteString(`Invalid mapping: {"generated":{"line":`)
	b.WriteString(strconv.Itoa(gen.Line) + `,"column":` + strconv.Itoa(gen.Column) + `}`)
	if m.Source != nil {
		b.WriteString(`,"source":` + js.QuoteJSON(*m.Source))
	} else {
		b.WriteString(`,"source":null`)
	}
	if m.Original != nil {
		b.WriteString(`,"original":{"line":` + strconv.Itoa(m.Original.Line) + `,"column":` +
			strconv.Itoa(m.Original.Column) + `}`)
	} else {
		b.WriteString(`,"original":null`)
	}
	if m.Name != nil {
		b.WriteString(`,"name":` + js.QuoteJSON(*m.Name) + `}`)
	} else {
		b.WriteString(`,"name":null}`)
	}
	return &InvalidMappingError{Message: b.String()}
}

// SetSourceContent is SourceMapGenerator.prototype.setSourceContent (the source root is always null in Less).
func (g *Generator) SetSourceContent(source string, content *string) {
	if content != nil {
		if g.sourcesContents == nil {
			g.sourcesContents = map[string]string{}
		}
		g.sourcesContents[source] = *content
		return
	}
	if g.sourcesContents != nil {
		delete(g.sourcesContents, source)
		if len(g.sourcesContents) == 0 {
			g.sourcesContents = nil
		}
	}
}

// JSON returns JSON.stringify(generator.toJSON()).
func (g *Generator) JSON() string {
	var b strings.Builder
	b.WriteString(`{"version":3,"sources":`)
	writeStrings(&b, g.sources.array)
	b.WriteString(`,"names":`)
	writeStrings(&b, g.names.array)
	b.WriteString(`,"mappings":` + js.QuoteJSON(g.serializeMappings()))
	if g.file != nil {
		b.WriteString(`,"file":` + js.QuoteJSON(*g.file))
	}
	if g.sourcesContents != nil {
		b.WriteString(`,"sourcesContent":[`)
		for i, s := range g.sources.array {
			if i > 0 {
				b.WriteByte(',')
			}
			if c, ok := g.sourcesContents[s]; ok {
				b.WriteString(js.QuoteJSON(c))
			} else {
				b.WriteString("null")
			}
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.String()
}

func writeStrings(b *strings.Builder, list []string) {
	b.WriteByte('[')
	for i, s := range list {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(js.QuoteJSON(s))
	}
	b.WriteByte(']')
}

func (g *Generator) serializeMappings() string {
	if !g.sorted {
		sort.SliceStable(g.mappings, func(i, j int) bool {
			return compareInflated(&g.mappings[i], &g.mappings[j]) < 0
		})
		g.sorted = true
	}
	prevGenColumn, prevGenLine, prevOrigColumn, prevOrigLine, prevName, prevSource := 0, 1, 0, 0, 0, 0
	var b strings.Builder
	for i := range g.mappings {
		m := &g.mappings[i]
		if m.generatedLine != prevGenLine {
			prevGenColumn = 0
			for m.generatedLine != prevGenLine {
				b.WriteByte(';')
				prevGenLine++
			}
		} else if i > 0 {
			if compareInflated(m, &g.mappings[i-1]) == 0 {
				continue
			}
			b.WriteByte(',')
		}
		b.WriteString(EncodeVLQ(m.generatedColumn - prevGenColumn))
		prevGenColumn = m.generatedColumn
		if m.source != nil {
			idx := g.sources.indexOf(*m.source)
			b.WriteString(EncodeVLQ(idx - prevSource))
			prevSource = idx
			b.WriteString(EncodeVLQ(m.originalLine - 1 - prevOrigLine))
			prevOrigLine = m.originalLine - 1
			b.WriteString(EncodeVLQ(m.originalColumn - prevOrigColumn))
			prevOrigColumn = m.originalColumn
			if m.name != nil {
				idx := g.names.indexOf(*m.name)
				b.WriteString(EncodeVLQ(idx - prevName))
				prevName = idx
			}
		}
	}
	return b.String()
}

// generatedPositionAfter is mapping-list.js generatedPositionAfter.
func generatedPositionAfter(a, b *entry) bool {
	return b.generatedLine > a.generatedLine ||
		b.generatedLine == a.generatedLine && b.generatedColumn >= a.generatedColumn ||
		compareInflated(a, b) <= 0
}

// compareInflated is util.compareByGeneratedPositionsInflated.
func compareInflated(a, b *entry) int {
	if c := a.generatedLine - b.generatedLine; c != 0 {
		return c
	}
	if c := a.generatedColumn - b.generatedColumn; c != 0 {
		return c
	}
	if c := strcmp(a.source, b.source); c != 0 {
		return c
	}
	if c := a.originalLine - b.originalLine; c != 0 {
		return c
	}
	if c := a.originalColumn - b.originalColumn; c != 0 {
		return c
	}
	return strcmp(a.name, b.name)
}

// strcmp is util.strcmp: null sorts after every string and strings compare by UTF-16 code units.
func strcmp(a, b *string) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	case *a == *b:
		return 0
	}
	ua, ub := utf16.Encode([]rune(*a)), utf16.Encode([]rune(*b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			if ua[i] > ub[i] {
				return 1
			}
			return -1
		}
	}
	if len(ua) > len(ub) {
		return 1
	}
	return -1
}

// arraySet is array-set.js with a native Map: an insertion-ordered set of strings.
type arraySet struct {
	array []string
	index map[string]int
}

func (s *arraySet) has(v string) bool {
	_, ok := s.index[v]
	return ok
}

func (s *arraySet) add(v string) {
	if s.index == nil {
		s.index = map[string]int{}
	}
	if _, ok := s.index[v]; !ok {
		s.index[v] = len(s.array)
		s.array = append(s.array, v)
	}
}

func (s *arraySet) indexOf(v string) int { return s.index[v] }

const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// EncodeVLQ is base64-vlq.js encode, with its 32-bit shift arithmetic.
func EncodeVLQ(value int) string {
	v := int32(value)
	var vlq uint32
	if v < 0 {
		vlq = uint32((-v)<<1) + 1
	} else {
		vlq = uint32(v << 1)
	}
	var b []byte
	for {
		digit := vlq & 31
		vlq >>= 5
		if vlq > 0 {
			digit |= 32
		}
		b = append(b, base64Chars[digit])
		if vlq == 0 {
			break
		}
	}
	return string(b)
}
