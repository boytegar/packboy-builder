package tool

import "testing"

func TestElidePath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"short stays", "/a/b/c.go", "/a/b/c.go"},
		{"long elides to last folder + name", "/home/user/proj/internal/app/conv/tool_render.go", ".../conv/tool_render.go"},
		{"long keeps filename even if two-seg tail long", "/home/user/proj/this/is/very/deep/directory/abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ.go", ".../abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ.go"},
		{"exactly maxLen stays", "/home/user/proj/internal/app/conv/tool_render.go"[:38], "/home/user/proj/internal/app/conv/tool_render.go"[:38]},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ElidePath(c.in); got != c.want {
				t.Errorf("ElidePath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
