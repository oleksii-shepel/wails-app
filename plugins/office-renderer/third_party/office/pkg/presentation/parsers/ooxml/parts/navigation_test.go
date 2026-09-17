package parts

import "testing"

func TestPackageNavigatorRejectsNilPackage(t *testing.T) {
	nav := newPackageNavigator(nil)
	if _, _, err := nav.relationships("ppt/slides/slide1.xml"); err == nil {
		t.Fatal("relationships on nil package must fail")
	}
	var slide Slide
	if _, err := nav.decodeXML("ppt/slides/slide1.xml", &slide); err == nil {
		t.Fatal("decodeXML on nil package must fail")
	}
}
