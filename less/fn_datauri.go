package less

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/less/less.js/internal/imagesize"
	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

var (
	mimeByExtension = map[string]string{}
	reMimeExtension = jsre.MustCompile(`^.*[\.\/\\]`, "")
	reTextCharset   = jsre.MustCompile(`^text\/|^application\/(javascript|json)`, "")
	reBase64Suffix  = jsre.MustCompile(`;base64$`, "")
)

func init() {
	// mime.define: every extension maps to the last type that lists it.
	for _, line := range strings.Split(mimeTypes, "\n") {
		fields := strings.Fields(line)
		for _, ext := range fields[1:] {
			mimeByExtension[ext] = fields[0]
		}
	}
	registerFunctions(map[string]func(*funcCaller, []Node) Node{
		"data-uri":     fnDataURI,
		"image-size":   func(fc *funcCaller, a []Node) Node { return imageSizeExpression(fc, a) },
		"image-width":  func(fc *funcCaller, a []Node) Node { return imageDimension(fc, a, true) },
		"image-height": func(fc *funcCaller, a []Node) Node { return imageDimension(fc, a, false) },
	})
}

// mimeLookup is mime 1.6.0 lookup: the text after the last dot or slash, lower-cased, with
// application/octet-stream for an unknown extension.
func mimeLookup(path string) string {
	ext := js.ToLowerCase(reMimeExtension.Replace(path, ""))
	if t, ok := mimeByExtension[ext]; ok {
		return t
	}
	return "application/octet-stream"
}

// charsetLookup is mime.charsets.lookup: text types are UTF-8, everything else has no charset.
func charsetLookup(mime string) string {
	if reTextCharset.Test(mime) {
		return "UTF-8"
	}
	return ""
}

// filePathArg reads `node.value` and the indexOf call the functions make on it, raising the TypeError the
// original raises when the argument is missing or its value is not a string.
func filePathArg(n Node) string {
	if n == nil {
		readUndefined("value")
	}
	s, ok := stringValue(n)
	if !ok {
		failStringValueAs(n, "filePath", "indexOf")
	}
	return s
}

// fileDirectory is the directory the functions resolve against: the current file's directory when URLs are
// rewritten, the entry file's directory otherwise.
func fileDirectory(fc *funcCaller) (string, bool) {
	fi := fc.fileInfo
	if fi == nil {
		return "", false
	}
	if fc.ctx.rewriteUrls != RewriteUrlsOff {
		return fi.CurrentDirectory, true
	}
	return fi.EntryPath, true
}

// checkFileManager reproduces the warnings environment.getFileManager prints for a missing name or directory.
func checkFileManager(filename string, haveDirectory bool) {
	if filename == "" {
		lessLogger.warn("getFileManager called with no filename.. Please report this issue. continuing.")
	}
	if !haveDirectory {
		lessLogger.warn("getFileManager called with null directory.. Please report this issue. continuing.")
	}
}

func dataURIFallback(fc *funcCaller, n Node) Node {
	return newURL(n, fc.index, fc.fileInfo, false).eval(fc.ctx)
}

// fnDataURI is functions/data-uri.js.
func fnDataURI(fc *funcCaller, args []Node) Node {
	mimetypeNode, filePathNode := argAt(args, 0), argAt(args, 1)
	if filePathNode == nil {
		filePathNode, mimetypeNode = mimetypeNode, nil
	}
	filePath := filePathArg(filePathNode)
	dir, haveDir := fileDirectory(fc)
	fragment := ""
	if i := js.IndexOf16(filePath, "#", 0); i != -1 {
		fragment = js.SliceJS(filePath, i, js.Len16(filePath))
		filePath = js.Slice16(filePath, 0, i)
	}
	checkFileManager(filePath, haveDir)
	var mimetype string
	useBase64 := false
	if mimetypeNode == nil {
		mimetype = mimeLookup(filePath)
		if mimetype != "image/svg+xml" {
			charset := charsetLookup(mimetype)
			useBase64 = charset != "US-ASCII" && charset != "UTF-8"
		}
		if useBase64 {
			mimetype += ";base64"
		}
	} else {
		mimetype = jsValueString(mimetypeNode)
		useBase64 = reBase64Suffix.Test(mimetype)
	}
	f, err := loadFileWith(filePath, dir, loadOptions{paths: fc.ctx.paths, rawBuffer: true})
	if err != nil {
		lessLogger.warn("Skipped data-uri embedding of " + filePath + " because file not found")
		return dataURIFallback(fc, filePathNode)
	}
	var buf string
	if useBase64 {
		buf = base64.StdEncoding.EncodeToString(f.raw)
	} else {
		enc, _ := js.EncodeURIComponent(js.DecodeUTF8(f.raw))
		buf = enc
	}
	uri := "data:" + mimetype + "," + buf + fragment
	return newURL(newQuoted("\""+uri+"\"", uri, false, fc.index, fc.fileInfo), fc.index, fc.fileInfo, false)
}

// imageSize is lib/less-node/image-size.js imageSize.
func imageSize(fc *funcCaller, args []Node) imagesize.Size {
	filePath := filePathArg(argAt(args, 0))
	dir, haveDir := fileDirectory(fc)
	if i := js.IndexOf16(filePath, "#", 0); i != -1 {
		filePath = js.Slice16(filePath, 0, i)
	}
	checkFileManager(filePath, haveDir)
	f, err := loadFileWith(filePath, dir, loadOptions{paths: fc.ctx.paths})
	if err != nil {
		panic(err)
	}
	data, rerr := os.ReadFile(f.filename)
	if rerr != nil {
		panic(&errorInfo{Type: "File", Message: rerr.Error()})
	}
	size, ok := imagesize.Probe(data)
	if !ok {
		panic(&errorInfo{Type: "File", Message: "Unrecognised image format for '" + filePath + "'"})
	}
	return size
}

func pxDimension(v float64) *Dimension { return newDimension(v, newUnit([]string{"px"}, nil, "")) }

func imageSizeExpression(fc *funcCaller, args []Node) Node {
	size := imageSize(fc, args)
	return newExpression([]Node{pxDimension(size.Width), pxDimension(size.Height)}, false)
}

func imageDimension(fc *funcCaller, args []Node, width bool) Node {
	size := imageSize(fc, args)
	if width {
		return pxDimension(size.Width)
	}
	return pxDimension(size.Height)
}
