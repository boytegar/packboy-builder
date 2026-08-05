package kit

import "testing"

func TestTruncateFilenameKeepExt(t *testing.T) {
	cases := []struct {
		name      string
		prefixLen int
		want      string
	}{
		{"inigambarsaya.png", 5, "iniga-.png"},
		{"hello.png", 5, "hello-.png"},      // already <= 5
		{"ab.png", 5, "ab-.png"},            // shorter than prefix
		{"verylongname.jpeg", 3, "ver-.jpeg"},
		{"noext", 5, "noext-"},              // no extension
		{"a.txt", 1, "a-.txt"},
		{"", 5, ""},                         // empty
		{"x.png", 0, "x.png"},               // prefixLen <= 0 → unchanged
		{"国际照片.png", 3, "国际照-.png"},     // rune-safe
		{".hidden", 5, ".hidd-"},          // dotfile: dot at 0 → no ext split, truncated as stem
	}
	for _, c := range cases {
		got := TruncateFilenameKeepExt(c.name, c.prefixLen)
		if got != c.want {
			t.Errorf("TruncateFilenameKeepExt(%q, %d) = %q, want %q", c.name, c.prefixLen, got, c.want)
		}
	}
}
