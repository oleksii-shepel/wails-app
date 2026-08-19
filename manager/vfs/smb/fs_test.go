package smb

import "testing"

func TestCleanUsesShareRelativePaths(t *testing.T) {
	fs := &FS{}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty root", in: "", want: "."},
		{name: "dot root", in: ".", want: "."},
		{name: "slash root", in: "/", want: "."},
		{name: "nested path", in: "/oleks/Downloads/unlocker", want: "oleks/Downloads/unlocker"},
		{name: "backslashes", in: `\oleks\Downloads\unlocker`, want: "oleks/Downloads/unlocker"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fs.clean(tc.in); got != tc.want {
				t.Fatalf("clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}