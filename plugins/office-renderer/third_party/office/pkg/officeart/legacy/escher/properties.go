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

	PropCropTop    uint16 = 0x0102
	PropCropBottom uint16 = 0x0103
	PropCropLeft   uint16 = 0x0104
	PropCropRight  uint16 = 0x0105

	PropFillType     uint16 = 0x0180
	PropFillColor    uint16 = 0x0181
	PropFillBooleans uint16 = 0x01BF

	PropFillBackColor uint16 = 0x0183

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
	PropLineBooleans         uint16 = 0x01FF

	PropShadowType     uint16 = 0x0200
	PropShadowColor    uint16 = 0x0201
	PropShadowOpacity  uint16 = 0x0204
	PropShadowOffsetX  uint16 = 0x0205
	PropShadowOffsetY  uint16 = 0x0206
	PropShadowSoftness uint16 = 0x021C
)
