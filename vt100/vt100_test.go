package vt100

import (
	"strings"
	"testing"
)

func TestVT100_UsedHeight(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    int
	}{
		{"empty screen", []byte(""), 0},
		{"line, line", []byte("Hello\nWorld\n"), 2},
		{"line, line, newline, newline", []byte("Hello\nWorld\n\n\n"), 2},
		{"line, space line, newline, newline", []byte("Hello\n World\n\n\n"), 2},
		{"line, newline, newline, line", []byte("Hello\n\n\nWorld\n"), 4},
		{"line, newline, newline, line, newline, newline", []byte("Hello\n\n\nWorld\n\n\n"), 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVT100(80, 10)
			v.Write(tt.content)
			if got := v.UsedHeight(); got != tt.want {
				t.Errorf("UsedHeight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVT100_Tail(t *testing.T) {
	tests := []struct {
		name    string
		content string
		lines   int
		want    string
	}{
		{
			"tail 1 on line, line, line, line",
			"Hello     \nWorld     \nFoo       \nBar       \n",
			1,
			"Bar       \n",
		},
		{
			"tail 2 on line, line, line, line",
			"Hello     \nWorld     \nFoo       \nBar       \n",
			2,
			"Foo       \nBar       \n",
		},
		{
			"tail 4 on line, line, line, line",
			"Hello     \nWorld     \nFoo       \nBar       \n",
			4,
			"Hello     \nWorld     \nFoo       \nBar       \n",
		},
		{
			"tail 6 on line, line, line, line",
			"Hello     \nWorld     \nFoo       \nBar       \n",
			6,
			"Hello     \nWorld     \nFoo       \nBar       \n",
		},
		{
			"tail -1 on line, line, line, line",
			"Hello     \nWorld     \nFoo       \nBar       \n",
			-1,
			"Hello     \nWorld     \nFoo       \nBar       \n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVT100(80, 10)

			for _, c := range strings.Split(tt.content, "\n") {
				v.Write([]byte(c))
			}
			got := ""
			for _, g := range v.Tail(tt.lines) {
				got += string(g) + "\n"
			}
			if got != tt.want {
				t.Errorf("Tail(%d) = %v, want %v", tt.lines, strings.Replace(got, "\n", "\\n", -1), strings.Replace(tt.want, "\n", "\\n", -1))
			}
		})
	}
}
