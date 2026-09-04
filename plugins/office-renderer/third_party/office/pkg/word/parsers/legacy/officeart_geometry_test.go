package legacy

import (
	"fmt"
	"testing"
)

func TestLegacyOfficeArtShapeTypeToGeometry(t *testing.T) {
	tests := []struct {
		shapeType uint16
		want      string
	}{
		{1, "rect"},
		{2, "roundRect"},
		{3, "ellipse"},
		{4, "diamond"},
		{5, "triangle"},
		{6, "rtTriangle"},
		{7, "parallelogram"},
		{8, "trapezoid"},
		{9, "hexagon"},
		{10, "octagon"},
		{11, "plus"},
		{12, "star5"},
		{13, "rightArrow"},
		{14, "leftArrow"},
		{15, "upArrow"},
		{16, "downArrow"},
		{17, "homePlate"},
		{18, "cube"},
		{22, "line"},
		{24, "can"},
		{25, "donut"},
		{34, "straightConnector1"},
		{35, "bentConnector2"},
		{36, "bentConnector3"},
		{37, "bentConnector4"},
		{38, "curvedConnector2"},
		{39, "curvedConnector3"},
		{40, "curvedConnector4"},
		{41, "callout1"},
		{42, "callout2"},
		{43, "callout3"},
		{44, "accentCallout1"},
		{45, "accentCallout2"},
		{46, "accentCallout3"},
		{47, "borderCallout1"},
		{48, "borderCallout2"},
		{49, "borderCallout3"},
		{50, "accentBorderCallout1"},
		{51, "accentBorderCallout2"},
		{52, "accentBorderCallout3"},
		{53, "ribbon"},
		{54, "ribbon2"},
		{55, "chevron"},
		{56, "pentagon"},
		{58, "star8"},
		{59, "star16"},
		{60, "star32"},
		{61, "wedgeRectCallout"},
		{62, "wedgeRoundRectCallout"},
		{63, "wedgeEllipseCallout"},
		{64, "wave"},
		{65, "foldedCorner"},
		{66, "leftRightArrow"},
		{67, "upDownArrow"},
		{68, "leftUpArrow"},
		{69, "leftRightUpArrow"},
		{70, "quadArrow"},
		{71, "leftRightArrowCallout"},
		{72, "upDownArrowCallout"},
		{73, "leftRightUpArrowCallout"},
		{74, "quadArrowCallout"},
		{75, "bentArrow"},
		{76, "uturnArrow"},
		{77, "circularArrow"},
		{78, "leftCircularArrow"},
		{79, "curvedLeftArrow"},
		{80, "curvedRightArrow"},
		{81, "curvedUpArrow"},
		{82, "curvedDownArrow"},
		{83, "cloudCallout"},
		{84, "cloud"},
		{88, "verticalScroll"},
		{89, "horizontalScroll"},
		{99, "funnel"},
		{100, "mathPlus"},
		{101, "mathMinus"},
		{102, "mathMultiply"},
		{103, "mathDivide"},
		{104, "mathEqual"},
		{105, "mathNotEqual"},
		{109, "heart"},
		{110, "lightningBolt"},
		{111, "sun"},
		{112, "moon"},
		{113, "smileyFace"},
		{117, "bevel"},
		{118, "frame"},
		{119, "halfFrame"},
		{120, "corner"},
		{122, "chord"},
		{124, "leftBracket"},
		{125, "rightBracket"},
		{126, "leftBrace"},
		{127, "rightBrace"},
		{128, "bracketPair"},
		{129, "bracePair"},
		{130, "straightConnector1"}, // duplicate IDs
		{131, "bentConnector2"},
		{132, "curvedConnector2"},
		// Unknown / unmapped
		{0, ""},
		{999, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("type_%d", tt.shapeType), func(t *testing.T) {
			got := legacyOfficeArtShapeTypeToGeometry(tt.shapeType)
			if got != tt.want {
				t.Errorf("legacyOfficeArtShapeTypeToGeometry(%d) = %q, want %q", tt.shapeType, got, tt.want)
			}
		})
	}
}