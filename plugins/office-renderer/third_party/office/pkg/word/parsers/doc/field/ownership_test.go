package field

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryReadersShareFieldGrammar(t *testing.T) {
	checks := []struct {
		path string
		want string
	}{
		{filepath.Join("..", "word97", "field_parse.go"), "docfield.ParseInstruction("},
		{filepath.Join("..", "pre97", "field_cursor.go"), "docfield.ParseInstruction("},
	}
	for _, check := range checks {
		data, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		source := string(data)
		if !strings.Contains(source, check.want) {
			t.Fatalf("%s must consume shared field grammar", check.path)
		}
		for _, forbidden := range []string{"func splitFieldTokens(", "func parseFieldInstruction("} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s retains duplicate field grammar: %q", check.path, forbidden)
			}
		}
	}
}
