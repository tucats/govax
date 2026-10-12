package rms

import "testing"

func TestMatchCase(t *testing.T) {
	cases := []struct{ model, word, whole, want string }{
		{"mar", "OBJ", "hello.mar", "obj"},
		{"MAR", "obj", "HELLO.MAR", "OBJ"},
		{"Mar", "OBJ", "Hello.Mar", "Obj"},
		{"mAR", "obj", "x.mAR", "oBJ"},
		{"m", "OBJ", "x.m", "obj"},
		{"M", "obj", "x.M", "OBJ"},
		{"", "OBJ", "prog", "obj"},
		{"", "obj", "Prog", "OBJ"},
	}

	for _, c := range cases {
		if got := MatchCase(c.model, c.word, c.whole); got != c.want {
			t.Errorf("MatchCase(%q, %q, %q) = %q, want %q", c.model, c.word, c.whole, got, c.want)
		}
	}
}
