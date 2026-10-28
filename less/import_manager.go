package less

// importManager is lib/less/import-manager.js: it records file contents for error messages and loads imports.
type importManager struct {
	contents             map[string]string
	contentsIgnoredChars map[string]int
	files                map[string]*importedFile
	fileOrder            []string
	rootFilename         string
	paths                []string
	queue                []string
	context              *parseContext
	err                  *errorInfo
	options              *Options
}

type importedFile struct {
	root    Node
	options *importOptions
	text    *string
}

func (m *importManager) contentsOf(filename string) (string, bool) {
	s, ok := m.contents[filename]
	return s, ok
}
