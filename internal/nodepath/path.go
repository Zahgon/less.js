// Package nodepath implements the POSIX functions of Node's path module that lessc uses, with Node's exact
// results for empty paths, trailing separators and "..".
package nodepath

import "strings"

// normalizeString is Node's normalizeString for the POSIX separator.
func normalizeString(path string, allowAboveRoot bool) string {
	res := ""
	lastSegmentLength, lastSlash, dots := 0, -1, 0
	var code byte
	for i := 0; i <= len(path); i++ {
		if i < len(path) {
			code = path[i]
		} else if code == '/' {
			break
		} else {
			code = '/'
		}
		if code == '/' {
			switch {
			case lastSlash == i-1 || dots == 1:
			case dots == 2:
				if len(res) < 2 || lastSegmentLength != 2 || !strings.HasSuffix(res, "..") {
					if len(res) > 2 {
						if j := strings.LastIndexByte(res, '/'); j == -1 {
							res, lastSegmentLength = "", 0
						} else {
							res = res[:j]
							lastSegmentLength = len(res) - 1 - strings.LastIndexByte(res, '/')
						}
						lastSlash, dots = i, 0
						continue
					} else if len(res) != 0 {
						res, lastSegmentLength, lastSlash, dots = "", 0, i, 0
						continue
					}
				}
				if allowAboveRoot {
					if len(res) > 0 {
						res += "/.."
					} else {
						res = ".."
					}
					lastSegmentLength = 2
				}
			default:
				if len(res) > 0 {
					res += "/" + path[lastSlash+1:i]
				} else {
					res = path[lastSlash+1 : i]
				}
				lastSegmentLength = i - lastSlash - 1
			}
			lastSlash, dots = i, 0
		} else if code == '.' && dots != -1 {
			dots++
		} else {
			dots = -1
		}
	}
	return res
}

// Resolve is path.resolve(cwd, ...paths) with the working directory given explicitly.
func Resolve(cwd string, paths ...string) string {
	all := append([]string{cwd}, paths...)
	resolved, absolute := "", false
	for i := len(all) - 1; i >= 0 && !absolute; i-- {
		p := all[i]
		if p == "" {
			continue
		}
		resolved = p + "/" + resolved
		absolute = p[0] == '/'
	}
	resolved = normalizeString(resolved, !absolute)
	if absolute {
		return "/" + resolved
	}
	if resolved == "" {
		return "."
	}
	return resolved
}

// Normalize is path.normalize.
func Normalize(path string) string {
	if path == "" {
		return "."
	}
	absolute := path[0] == '/'
	trailing := path[len(path)-1] == '/'
	path = normalizeString(path, !absolute)
	if path == "" {
		if absolute {
			return "/"
		}
		if trailing {
			return "./"
		}
		return "."
	}
	if trailing {
		path += "/"
	}
	if absolute {
		return "/" + path
	}
	return path
}

// Join is path.join.
func Join(paths ...string) string {
	var parts []string
	for _, p := range paths {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "."
	}
	return Normalize(strings.Join(parts, "/"))
}

// Relative is path.relative(from, to), resolving both against cwd.
func Relative(cwd, from, to string) string {
	if from == to {
		return ""
	}
	from, to = Resolve(cwd, from), Resolve(cwd, to)
	if from == to {
		return ""
	}
	fromLen, toLen := len(from)-1, len(to)-1
	length := min(fromLen, toLen)
	lastCommonSep, i := -1, 0
	for ; i < length; i++ {
		c := from[1+i]
		if c != to[1+i] {
			break
		} else if c == '/' {
			lastCommonSep = i
		}
	}
	if i == length {
		if toLen > length {
			if to[1+i] == '/' {
				return to[1+i+1:]
			}
			if i == 0 {
				return to[1+i:]
			}
		} else if fromLen > length {
			if from[1+i] == '/' {
				lastCommonSep = i
			} else if i == 0 {
				lastCommonSep = 0
			}
		}
	}
	out := ""
	for i = 1 + lastCommonSep + 1; i <= len(from); i++ {
		if i == len(from) || from[i] == '/' {
			if out == "" {
				out = ".."
			} else {
				out += "/.."
			}
		}
	}
	return out + to[1+lastCommonSep:]
}

// Dirname is path.dirname.
func Dirname(path string) string {
	if path == "" {
		return "."
	}
	hasRoot := path[0] == '/'
	end, matchedSlash := -1, true
	for i := len(path) - 1; i >= 1; i-- {
		if path[i] == '/' {
			if !matchedSlash {
				end = i
				break
			}
		} else {
			matchedSlash = false
		}
	}
	if end == -1 {
		if hasRoot {
			return "/"
		}
		return "."
	}
	if hasRoot && end == 1 {
		return "//"
	}
	return path[:end]
}

// Basename is path.basename without an extension argument.
func Basename(path string) string {
	start, end, matchedSlash := 0, -1, true
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if !matchedSlash {
				start = i + 1
				break
			}
		} else if end == -1 {
			matchedSlash = false
			end = i + 1
		}
	}
	if end == -1 {
		return ""
	}
	return path[start:end]
}
