package resolve

import "testing"

func TestSmartArtPicturePlaceholderPresentationIDs(t *testing.T) {
	xml := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst><dgm:pt modelId="img" type="pres"><dgm:prSet presName="imgShp" presStyleLbl="fgImgPlace1"/></dgm:pt><dgm:pt modelId="text" type="pres"><dgm:prSet presName="txShp" presStyleLbl="node1"/></dgm:pt></dgm:ptLst></dgm:dataModel>`)
	ids := smartArtPicturePlaceholderPresentationIDs(xml)
	if !ids["img"] || ids["text"] {
		t.Fatalf("ids = %#v", ids)
	}
}
