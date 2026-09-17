package parts

import (
	"os"
	"strings"
	"testing"
)

func TestLoadPackageDelegatesTopLevelOwnershipToAssembler(t *testing.T) {
	data, err := os.ReadFile("load.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	bodyStart := strings.Index(src, "func LoadPackage(")
	bodyEnd := strings.Index(src[bodyStart:], "\nfunc ")
	if bodyStart < 0 || bodyEnd < 0 {
		t.Fatal("LoadPackage body not found")
	}
	body := src[bodyStart : bodyStart+bodyEnd]
	for _, required := range []string{"newPackageAssembler(pkg)", "assembler.assemble()"} {
		if !strings.Contains(body, required) {
			t.Fatalf("LoadPackage must delegate through %q", required)
		}
	}
	for _, forbidden := range []string{"loadSlides(", "loadSlideMasters(", "loadThemes(", "pkg.Relationships("} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("LoadPackage retained top-level assembly responsibility %q", forbidden)
		}
	}
}
