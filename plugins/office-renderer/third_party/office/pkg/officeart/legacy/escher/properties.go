package escher

// Common OfficeArt property identifiers used across DOC/XLS/PPT.
//
// Keep application-specific interpretation outside this package unless the
// property semantics are defined by OfficeArt itself.
const (
	PropRotation uint16 = 0x0004

	PropTextLeft     uint16 = 0x0081
	PropTextTop      uint16 = 0x0082
	PropTextRight    uint16 = 0x0083
	PropTextBottom   uint16 = 0x0084
	PropWrapText     uint16 = 0x0085
	PropAnchorText   uint16 = 0x0087
	PropTextFlow     uint16 = 0x0088
	PropTextBooleans uint16 = 0x00BF
	// Geometry text / WordArt properties ([MS-ODRAW]).
	PropGTextUnicode  uint16 = 0x00C0
	PropGTextAlign    uint16 = 0x00C2
	PropGTextSize     uint16 = 0x00C3
	PropGTextSpacing  uint16 = 0x00C4
	PropGTextFont     uint16 = 0x00C5
	PropGTextBooleans uint16 = 0x00FF

	// OfficeArt Blip crop properties (MS-ODRAW 2.3.23.1-2.3.23.4).
	// Keep these numeric IDs exact: 0x0104 is pib, not a crop edge.
	PropCropTop    uint16 = 0x0100
	PropCropBottom uint16 = 0x0101
	PropCropLeft   uint16 = 0x0102
	PropCropRight  uint16 = 0x0103

	PropPictureContrast   uint16 = 0x0108
	PropPictureBrightness uint16 = 0x0109
	PropPictureGray       uint16 = 0x013D
	PropPictureBiLevel    uint16 = 0x013E
	PropPictureBooleans   uint16 = 0x013F

	PropGeoLeft     uint16 = 0x0140
	PropGeoTop      uint16 = 0x0141
	PropGeoRight    uint16 = 0x0142
	PropGeoBottom   uint16 = 0x0143
	PropShapePath   uint16 = 0x0144
	PropVertices    uint16 = 0x0145
	PropSegmentInfo uint16 = 0x0146
	PropAdjust1     uint16 = 0x0147
	PropAdjust2     uint16 = 0x0148
	PropAdjust3     uint16 = 0x0149
	PropAdjust4     uint16 = 0x014A
	PropAdjust5     uint16 = 0x014B
	PropAdjust6     uint16 = 0x014C
	PropAdjust7     uint16 = 0x014D
	PropAdjust8     uint16 = 0x014E

	PropFillType     uint16 = 0x0180
	PropFillColor    uint16 = 0x0181
	PropFillOpacity  uint16 = 0x0182
	PropFillBooleans uint16 = 0x01BF

	PropFillBlip        uint16 = 0x0186
	PropFillBackColor   uint16 = 0x0183
	PropFillAngle       uint16 = 0x018B
	PropFillFocus       uint16 = 0x018C
	PropFillToLeft      uint16 = 0x018D
	PropFillToTop       uint16 = 0x018E
	PropFillToRight     uint16 = 0x018F
	PropFillToBottom    uint16 = 0x0190
	PropFillShadeColors uint16 = 0x0197

	PropLineColor            uint16 = 0x01C0
	PropLineOpacity          uint16 = 0x01C7
	PropLineWidth            uint16 = 0x01CB
	PropLineDashing          uint16 = 0x01CE
	PropLineStartArrow       uint16 = 0x01D0
	PropLineEndArrow         uint16 = 0x01D1
	PropLineStartArrowWidth  uint16 = 0x01D2
	PropLineStartArrowLength uint16 = 0x01D3
	PropLineEndArrowWidth    uint16 = 0x01D4
	PropLineEndArrowLength   uint16 = 0x01D5
	PropLineJoinStyle        uint16 = 0x01D6
	PropLineEndCapStyle      uint16 = 0x01D7
	PropLineBooleans         uint16 = 0x01FF

	PropShadowType     uint16 = 0x0200
	PropShadowColor    uint16 = 0x0201
	PropShadowOpacity  uint16 = 0x0204
	PropShadowOffsetX  uint16 = 0x0205
	PropShadowOffsetY  uint16 = 0x0206
	PropShadowSoftness uint16 = 0x021C
	PropShadowBooleans uint16 = 0x023F
)
