package nodepath

import "testing"

// The expected values are what Node's path module returns for the same arguments.
func TestPathFunctions(t *testing.T) {
	cwd := "/tmp/c/w"
	cases := []struct{ fn, a, b, want string }{
		{"resolve", "", "", "/tmp/c/w"},
		{"resolve", "../x/", "", "/tmp/c/x"},
		{"resolve", "/a//b/../c/", "", "/a/c"},
		{"normalize", "./", "", "./"},
		{"normalize", "a/../..", "", ".."},
		{"normalize", "//a/.", "", "/a"},
		{"join", "", "", "."},
		{"join", "a/", "../b/", "b/"},
		{"join", "/", "..", "/"},
		{"relative", "/tmp/c/w/out", "/tmp/c/w/maps", "../maps"},
		{"relative", "/tmp/c/w", "/tmp/c/w", ""},
		{"relative", "/a/bc", "/a/b", "../b"},
		{"dirname", "/a/b/", "", "/a"},
		{"dirname", "a", "", "."},
		{"dirname", "//x", "", "//"},
		{"basename", "/a/b.css/", "", "b.css"},
		{"basename", "/", "", ""},
	}
	for _, c := range cases {
		var got string
		switch c.fn {
		case "resolve":
			got = Resolve(cwd, c.a)
		case "normalize":
			got = Normalize(c.a)
		case "join":
			got = Join(c.a, c.b)
		case "relative":
			got = Relative(cwd, c.a, c.b)
		case "dirname":
			got = Dirname(c.a)
		case "basename":
			got = Basename(c.a)
		}
		if got != c.want {
			t.Errorf("%s(%q, %q) = %q, want %q", c.fn, c.a, c.b, got, c.want)
		}
	}
}
