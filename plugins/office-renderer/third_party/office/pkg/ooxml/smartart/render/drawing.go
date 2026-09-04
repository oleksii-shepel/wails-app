package render

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/smartart"
)

// bodyPr captures the text-body properties we care about from a:bodyPr.
type bodyPr struct {
	Anchor      string    `xml:"anchor,attr"`
	AnchorCtr   string    `xml:"anchorCtr,attr"`
	Rot         int       `xml:"rot,attr"`
	Wrap        string    `xml:"wrap,attr"`
	Vert        string    `xml:"vert,attr"`
	LIns        *int64    `xml:"lIns,attr"`
	RIns        *int64    `xml:"rIns,attr"`
	TIns        *int64    `xml:"tIns,attr"`
	BIns        *int64    `xml:"bIns,attr"`
	NoAutofit   *struct{} `xml:"noAutofit"`
	SpAutoFit   *struct{} `xml:"spAutoFit"`
	NormAutofit *struct {
		FontScale          int `xml:"fontScale,attr"`
		LineSpaceReduction int `xml:"lnSpcReduction,attr"`
	} `xml:"normAutofit"`
}

func bodyPrToProperties(bp *bodyPr) drawing.TextBodyProperties {
	if bp == nil {
		return drawing.TextBodyProperties{}
	}
	props := drawing.TextBodyProperties{
		Anchor:    bp.Anchor,
		AnchorCtr: bp.AnchorCtr == "1",
		Rot:       bp.Rot,
		Wrap:      bp.Wrap,
		Vert:      bp.Vert,
	}
	if bp.LIns != nil {
		props.InsetL = *bp.LIns
	}
	if bp.RIns != nil {
		props.InsetR = *bp.RIns
	}
	if bp.TIns != nil {
		props.InsetT = *bp.TIns
	}
	if bp.BIns != nil {
		props.InsetB = *bp.BIns
	}
	switch {
	case bp.NoAutofit != nil:
		props.AutofitType = "noAutofit"
	case bp.SpAutoFit != nil:
		props.AutofitType = "spAutoFit"
	case bp.NormAutofit != nil:
		props.AutofitType = "normAutofit"
		props.AutofitFontScale = bp.NormAutofit.FontScale
		props.AutofitLineSpaceReduction = bp.NormAutofit.LineSpaceReduction
	}
	return props
}

// DrawingShapeInfo holds the parsed geometry and appearance of a single shape
// from a pre-rendered dsp:drawing part.
type DrawingShapeInfo struct {
	// ModelID is the presentation node identifier from the drawing part.
	ModelID string

	// X, Y, W, H are the shape bounds in EMU.
	X, Y, W, H float64

	// RotationDeg is the clockwise rotation of the shape in degrees.
	RotationDeg float64

	// FlipH and FlipV reflect horizontal/vertical flips.
	FlipH bool
	FlipV bool

	// Geometry is the DrawingML preset geometry name (e.g. "rect", "ellipse").
	Geometry string

	// Adjustments holds preset-geometry adjustment values (0-1) extracted from
	// a:avLst, in declaration order.
	Adjustments []float64

	// GeometryData holds a parsed a:custGeom custom geometry, if any.
	GeometryData *drawing.Geometry

	// DrawingOrder is the position of this shape in the source dsp:spTree,
	// used to preserve front-to-back ordering.
	DrawingOrder int

	// Fill is the resolved fill, or nil when not present.
	Fill drawing.Fill

	// Line is the resolved line style, or nil when not present.
	Line *drawing.LineStyle

	// Effect is the resolved effect list (shadows, glows, 3D, etc.).
	Effect drawing.EffectList

	// Text is the plain text extracted from the shape's txBody.
	Text string

	// Paragraphs holds styled text runs when the drawing part provides them.
	Paragraphs []*smartart.TextParagraph

	// TextBodyProps holds text-body properties for the shape's text container.
	TextBodyProps drawing.TextBodyProperties

	// SourceID is the non-visual shape id (cNvPr@id) from the drawing part.
	// It is used to validate connector endpoint references when the drawing
	// supplies meaningful ids.
	SourceID int
}

// DrawingConnectorInfo holds the parsed geometry and style of a pre-rendered
// dsp:cxnSp connector.  When available, its bounds and connector preset are
// used to compute the exact connector path instead of synthesizing straight or
// elbow segments.
type DrawingConnectorInfo struct {
	// Bounds in EMU.
	X, Y, W, H float64

	// RotationDeg is the clockwise rotation of the connector in degrees.
	RotationDeg float64

	// FlipH and FlipV reflect horizontal/vertical flips.
	FlipH bool
	FlipV bool

	// Geometry is the DrawingML connector preset name (e.g. "bentConnector3").
	Geometry string

	// Adjustments holds connector adjustment values (0-1) from a:avLst.
	Adjustments []float64

	// Line is the resolved line style for the connector.
	Line *drawing.LineStyle

	// Connection-site references, when supplied by the drawing part.
	StCxnID, EndCxnID   int
	StCxnIdx, EndCxnIdx int

	// DrawingOrder is the position of this connector in the source dsp:spTree.
	DrawingOrder int
}

// parseDiagramDrawing parses a dsp:drawing XML part and returns the shapes it
// contains, keyed by their modelId attribute.  When themeColors is supplied,
// schemeClr fill/line values are resolved to hex RGB colors and alpha
// multipliers are applied. Group shapes (dsp:grpSp) are recursively flattened
// and their transforms are composed onto child shapes.
//
// The parser is token-based so it can handle arbitrarily deep group nesting
// without loading the entire drawing into a single DOM.
func parseDiagramDrawing(drawingXML []byte, themeColors map[string]string) (map[string]*DrawingShapeInfo, *drawing.LineStyle, error) {
	shapes, _, connStyle, err := parseDiagramDrawingWithConnectors(drawingXML, themeColors)
	return shapes, connStyle, err
}

// parseDiagramDrawingWithConnectors parses a dsp:drawing XML part and returns
// the shapes, pre-rendered connectors, and the first connector line style it
// contains.
func parseDiagramDrawingWithConnectors(drawingXML []byte, themeColors map[string]string) (map[string]*DrawingShapeInfo, []*DrawingConnectorInfo, *drawing.LineStyle, error) {
	if len(drawingXML) == 0 {
		return nil, nil, nil, nil
	}

	dec := xml.NewDecoder(bytes.NewReader(drawingXML))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local == "drawing" {
			return parseDrawing(dec, themeColors)
		}
		if err := skipElement(dec, se); err != nil {
			return nil, nil, nil, err
		}
	}
	return nil, nil, nil, nil
}

// parseDrawing walks the children of dsp:drawing looking for dsp:spTree.
func parseDrawing(dec *xml.Decoder, themeColors map[string]string) (map[string]*DrawingShapeInfo, []*DrawingConnectorInfo, *drawing.LineStyle, error) {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "spTree" {
				return parseSpTree(dec, themeColors)
			}
			if err := skipElement(dec, se); err != nil {
				return nil, nil, nil, err
			}
		case xml.EndElement:
			if se.Name.Local == "drawing" {
				return nil, nil, nil, nil
			}
		}
	}
	return nil, nil, nil, nil
}

// parseSpTree walks the children of dsp:spTree, decoding shapes, connectors,
// and groups as it encounters them.
func parseSpTree(dec *xml.Decoder, themeColors map[string]string) (map[string]*DrawingShapeInfo, []*DrawingConnectorInfo, *drawing.LineStyle, error) {
	shapes := make(map[string]*DrawingShapeInfo)
	var connectors []*DrawingConnectorInfo
	var connStyle *drawing.LineStyle
	order := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "sp":
				var sp drawingSp
				if err := dec.DecodeElement(&sp, &se); err != nil {
					return nil, nil, nil, err
				}
				if info := drawingShapeFromSp(sp, themeColors); info != nil {
					info.DrawingOrder = order
					order++
					shapes[info.ModelID] = info
				}
			case "cxnSp":
				var cxn drawingCxnSp
				if err := dec.DecodeElement(&cxn, &se); err != nil {
					return nil, nil, nil, err
				}
				if info := drawingConnectorFromCxnSp(cxn, themeColors); info != nil {
					info.DrawingOrder = order
					order++
					connectors = append(connectors, info)
					if connStyle == nil {
						connStyle = info.Line
					}
				}
			case "grpSp":
				var grp drawingGrpSp
				if err := dec.DecodeElement(&grp, &se); err != nil {
					return nil, nil, nil, err
				}
				if s := drawingProcessGroup(grp, nil, shapes, &connectors, themeColors, &order); s != nil && connStyle == nil {
					connStyle = s
				}
			default:
				if err := skipElement(dec, se); err != nil {
					return nil, nil, nil, err
				}
			}
		case xml.EndElement:
			if se.Name.Local == "spTree" {
				return shapes, connectors, connStyle, nil
			}
		}
	}
	return shapes, connectors, connStyle, nil
}

// skipElement advances the decoder past the end of the current element.
func skipElement(dec *xml.Decoder, se xml.StartElement) error {
	return dec.Skip()
}

// drawingColorInfo captures a color value and optional alpha multiplier.
type drawingColorInfo struct {
	Val   string `xml:"val,attr"`
	Alpha *struct {
		Val int64 `xml:"val,attr"`
	} `xml:"alpha"`
	AlphaMod *struct {
		Val int64 `xml:"val,attr"`
	} `xml:"alphaMod"`
	AlphaOff *struct {
		Val int64 `xml:"val,attr"`
	} `xml:"alphaOff"`
}

// drawingXfrm captures a DrawingML 2-D transform, including the child
// coordinate system used by groups.
type drawingXfrm struct {
	Off   drawingOffset  `xml:"off"`
	Ext   drawingExtents `xml:"ext"`
	ChOff drawingOffset  `xml:"chOff"`
	ChExt drawingExtents `xml:"chExt"`
	Rot   int            `xml:"rot,attr,omitempty"`
	FlipH bool           `xml:"flipH,attr,omitempty"`
	FlipV bool           `xml:"flipV,attr,omitempty"`
}

type drawingOffset struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

type drawingExtents struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

// drawingDoc is the top-level structure of a dsp:drawing part.
type drawingDoc struct {
	SpTree struct {
		Sp    []drawingSp    `xml:"sp"`
		CxnSp []drawingCxnSp `xml:"cxnSp"`
		GrpSp []drawingGrpSp `xml:"grpSp"`
	} `xml:"spTree"`
}

// drawingSpPr captures shape properties from dsp:spPr.
type drawingSpPr struct {
	Xfrm     drawingXfrm `xml:"xfrm"`
	PrstGeom struct {
		Prst  string `xml:"prst,attr"`
		AvLst struct {
			Gd []struct {
				Name string `xml:"name,attr"`
				Fmla string `xml:"fmla,attr"`
			} `xml:"gd"`
		} `xml:"avLst"`
	} `xml:"prstGeom"`
	CustGeom  *drawingxml.CustomGeometry `xml:"custGeom"`
	NoFill    *struct{}                  `xml:"noFill"`
	SolidFill drawingSolidFill           `xml:"solidFill"`
	GradFill  *drawingGradFill           `xml:"gradFill"`
	PattFill  *drawingPattFill           `xml:"pattFill"`
	BlipFill  *drawingBlipFill           `xml:"blipFill"`
	EffectLst *drawingEffectList         `xml:"effectLst"`
	Scene3D   *drawingScene3D            `xml:"scene3d"`
	Sp3D      *drawingSp3D               `xml:"sp3d"`
	Ln        *drawingLn                 `xml:"ln"`
}

type drawingSolidFill struct {
	SrgbClr   *drawingColorInfo `xml:"srgbClr"`
	SchemeClr *drawingColorInfo `xml:"schemeClr"`
}

type drawingGradFill struct {
	GsLst []struct {
		Pos       int64             `xml:"pos,attr"`
		SrgbClr   *drawingColorInfo `xml:"srgbClr"`
		SchemeClr *drawingColorInfo `xml:"schemeClr"`
	} `xml:"gsLst>gs"`
	Lin *struct {
		Ang int64 `xml:"ang,attr"`
	} `xml:"lin"`
}

type drawingColorRef struct {
	SrgbClr   *drawingColorInfo `xml:"srgbClr"`
	SchemeClr *drawingColorInfo `xml:"schemeClr"`
}

func clampPictureFraction(value int) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100000 {
		return 1
	}
	return float64(value) / 100000.0
}

func maxPictureFraction(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func drawingColorRefString(ref *drawingColorRef, themeColors map[string]string) string {
	if ref == nil {
		return ""
	}
	if ref.SrgbClr != nil {
		color, _ := resolveDrawingColor(ref.SrgbClr)
		return color
	}
	if ref.SchemeClr != nil && themeColors != nil {
		if c, ok := themeColors[ref.SchemeClr.Val]; ok {
			return c
		}
	}
	return ""
}

type drawingBlipFill struct {
	DPI          int    `xml:"dpi,attr"`
	RotWithShape string `xml:"rotWithShape,attr"`
	Blip         struct {
		Embed   string    `xml:"embed,attr"`
		Link    string    `xml:"link,attr"`
		Grayscl *struct{} `xml:"grayscl"`
		BiLevel *struct {
			Thresh int `xml:"thresh,attr"`
		} `xml:"biLevel"`
		Duotone *struct {
			Clr1 drawingColorRef `xml:"clr1"`
			Clr2 drawingColorRef `xml:"clr2"`
		} `xml:"duotone"`
		Lum *struct {
			Bright   int `xml:"bright,attr"`
			Contrast int `xml:"contrast,attr"`
		} `xml:"lum"`
		AlphaMod *struct {
			Amt int `xml:"amt,attr"`
		} `xml:"alphaMod"`
		AlphaModFix *struct {
			Amt int `xml:"amt,attr"`
		} `xml:"alphaModFix"`
		AlphaOff *struct {
			Amt int `xml:"amt,attr"`
		} `xml:"alphaOff"`
		AlphaRepl *struct {
			A int `xml:"a,attr"`
		} `xml:"alphaRepl"`
		AlphaBiLevel *struct {
			Thresh int `xml:"thresh,attr"`
		} `xml:"alphaBiLevel"`
		AlphaOutset *struct {
			Rad int64 `xml:"rad,attr"`
		} `xml:"alphaOutset"`
		AlphaInv     *struct{} `xml:"alphaInv"`
		AlphaCeiling *struct{} `xml:"alphaCeiling"`
		AlphaFloor   *struct{} `xml:"alphaFloor"`
		HSL          *struct {
			Hue int `xml:"hue,attr"`
			Sat int `xml:"sat,attr"`
			Lum int `xml:"lum,attr"`
		} `xml:"hsl"`
		ClrRepl   *drawingColorRef `xml:"clrRepl"`
		ClrChange *struct {
			UseAlpha bool            `xml:"useA,attr"`
			From     drawingColorRef `xml:"clrFrom"`
			To       drawingColorRef `xml:"clrTo"`
		} `xml:"clrChange"`
	} `xml:"blip"`
	SrcRect *struct {
		L int `xml:"l,attr"`
		T int `xml:"t,attr"`
		R int `xml:"r,attr"`
		B int `xml:"b,attr"`
	} `xml:"srcRect"`
	Stretch *struct {
		FillRect *struct {
			L int `xml:"l,attr"`
			T int `xml:"t,attr"`
			R int `xml:"r,attr"`
			B int `xml:"b,attr"`
		} `xml:"fillRect"`
	} `xml:"stretch"`
	Tile *struct {
		Tx   int64  `xml:"tx,attr"`
		Ty   int64  `xml:"ty,attr"`
		Sx   int    `xml:"sx,attr"`
		Sy   int    `xml:"sy,attr"`
		Flip string `xml:"flip,attr"`
		Algn string `xml:"algn,attr"`
	} `xml:"tile"`
}

type drawingPattFill struct {
	Prst  string            `xml:"prst,attr"`
	FgClr *drawingSolidFill `xml:"fgClr"`
	BgClr *drawingSolidFill `xml:"bgClr"`
}

type drawingEffectList struct {
	InnerXML    string              `xml:",innerxml"`
	OuterShadow *drawingOuterShadow `xml:"outerShdw"`
	InnerShadow *drawingInnerShadow `xml:"innerShdw"`
	Glow        *drawingGlow        `xml:"glow"`
	SoftEdge    *drawingSoftEdge    `xml:"softEdge"`
	Reflection  *drawingReflection  `xml:"reflection"`
	Blur        *drawingBlur        `xml:"blur"`
}

type drawingOuterShadow struct {
	BlurRad   int64             `xml:"blurRad,attr,omitempty"`
	Dist      int64             `xml:"dist,attr,omitempty"`
	Dir       int               `xml:"dir,attr,omitempty"`
	SrgbClr   *drawingColorInfo `xml:"srgbClr"`
	SchemeClr *drawingColorInfo `xml:"schemeClr"`
}

type drawingInnerShadow struct {
	BlurRad   int64             `xml:"blurRad,attr,omitempty"`
	Dist      int64             `xml:"dist,attr,omitempty"`
	Dir       int               `xml:"dir,attr,omitempty"`
	SrgbClr   *drawingColorInfo `xml:"srgbClr"`
	SchemeClr *drawingColorInfo `xml:"schemeClr"`
}

type drawingGlow struct {
	Rad       int64             `xml:"rad,attr,omitempty"`
	SrgbClr   *drawingColorInfo `xml:"srgbClr"`
	SchemeClr *drawingColorInfo `xml:"schemeClr"`
}

type drawingSoftEdge struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

type drawingReflection struct {
	BlurRad int64 `xml:"blurRad,attr,omitempty"`
	Dist    int64 `xml:"dist,attr,omitempty"`
	Dir     int   `xml:"dir,attr,omitempty"`
	StA     int   `xml:"stA,attr,omitempty"`
}

type drawingBlur struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

type drawingScene3D struct {
	Camera   *drawingCamera   `xml:"camera"`
	LightRig *drawingLightRig `xml:"lightRig"`
}

type drawingCamera struct {
	Prst string           `xml:"prst,attr,omitempty"`
	FOV  int64            `xml:"fov,attr,omitempty"`
	Zoom int64            `xml:"zoom,attr,omitempty"`
	Rot  *drawingSceneRot `xml:"rot"`
}

type drawingSceneRot struct {
	Lat int64 `xml:"lat,attr,omitempty"`
	Lon int64 `xml:"lon,attr,omitempty"`
	Rev int64 `xml:"rev,attr,omitempty"`
}

type drawingLightRig struct {
	Rig string           `xml:"rig,attr,omitempty"`
	Dir string           `xml:"dir,attr,omitempty"`
	Rot *drawingSceneRot `xml:"rot"`
}

type drawingSp3D struct {
	Z             int64             `xml:"z,attr,omitempty"`
	ExtrusionH    int64             `xml:"extrusionH,attr,omitempty"`
	ContourW      int64             `xml:"contourW,attr,omitempty"`
	PrstMaterial  string            `xml:"prstMaterial,attr,omitempty"`
	BevelT        *drawingBevel     `xml:"bevelT"`
	BevelB        *drawingBevel     `xml:"bevelB"`
	ContourClr    *drawingColorInfo `xml:"contourClr>srgbClr"`
	ContourScheme *drawingColorInfo `xml:"contourClr>schemeClr"`
	ExtrusionClr  *drawingColorInfo `xml:"extrusionClr>srgbClr"`
	ExtrudeScheme *drawingColorInfo `xml:"extrusionClr>schemeClr"`
}

type drawingBevel struct {
	W    int64  `xml:"w,attr,omitempty"`
	H    int64  `xml:"h,attr,omitempty"`
	Prst string `xml:"prst,attr,omitempty"`
}

type drawingLn struct {
	W         int64            `xml:"w,attr"`
	SolidFill drawingSolidFill `xml:"solidFill"`
	GradFill  *drawingGradFill `xml:"gradFill"`
	Cap       string           `xml:"cap,attr"`
	Cmpd      string           `xml:"cmpd,attr"`
	PrstDash  *struct {
		Val string `xml:"val,attr"`
	} `xml:"prstDash"`
	HeadEnd *drawingEnd `xml:"headEnd"`
	TailEnd *drawingEnd `xml:"tailEnd"`
}

type drawingEnd struct {
	Type string `xml:"type,attr"`
	W    string `xml:"w,attr"`
	Len  string `xml:"len,attr"`
}

// drawingSp captures a single dsp:sp shape.
type drawingSp struct {
	ModelID string `xml:"modelId,attr"`
	NvSpPr  struct {
		CNvPr struct {
			ID   int    `xml:"id,attr"`
			Name string `xml:"name,attr"`
		} `xml:"cNvPr"`
	} `xml:"nvSpPr"`
	SpPr   drawingSpPr   `xml:"spPr"`
	TxBody drawingTxBody `xml:"txBody"`
}

type drawingTxBody struct {
	BodyPr *bodyPr       `xml:"bodyPr"`
	P      []drawingPara `xml:"p"`
}

type drawingSpacingDef struct {
	SpcPts *struct {
		Val int `xml:"val,attr"`
	} `xml:"spcPts"`
	SpcPct *struct {
		Val int `xml:"val,attr"`
	} `xml:"spcPct"`
}

type drawingRPr struct {
	B         string           `xml:"b,attr"`
	I         string           `xml:"i,attr"`
	U         string           `xml:"u,attr"`
	Sz        int64            `xml:"sz,attr"`
	SldFill   drawingSolidFill `xml:"solidFill"`
	EffectLst *struct {
		InnerXML string `xml:",innerxml"`
	} `xml:"effectLst"`
	Latin *struct {
		Typeface string `xml:"typeface,attr"`
	} `xml:"latin"`
	EA *struct {
		Typeface string `xml:"typeface,attr"`
	} `xml:"ea"`
	CS *struct {
		Typeface string `xml:"typeface,attr"`
	} `xml:"cs"`
}

type drawingPara struct {
	PPr *struct {
		Algn     string             `xml:"algn,attr"`
		Lvl      int                `xml:"lvl,attr,omitempty"`
		DefTabSz int                `xml:"defTabSz,attr,omitempty"`
		SpcBef   *drawingSpacingDef `xml:"spcBef"`
		SpcAft   *drawingSpacingDef `xml:"spcAft"`
		LnSpc    *drawingSpacingDef `xml:"lnSpc"`
		TabStops []struct {
			Pos  int64  `xml:"pos,attr"`
			Algn string `xml:"algn,attr"`
		} `xml:"tabLst>tab"`
		BuChar *struct {
			Char string `xml:"char,attr"`
		} `xml:"buChar"`
		BuAutoNum *struct {
			Type    string `xml:"type,attr"`
			StartAt int    `xml:"startAt,attr,omitempty"`
		} `xml:"buAutoNum"`
		BuNone      *struct{}         `xml:"buNone"`
		BuClr       *drawingColorInfo `xml:"buClr>srgbClr"`
		BuClrScheme *drawingColorInfo `xml:"buClr>schemeClr"`
		BuFont      *struct {
			Typeface string `xml:"typeface,attr"`
		} `xml:"buFont"`
		BuSzPts *struct {
			Val int `xml:"val,attr"`
		} `xml:"buSzPts"`
		BuSzPct *struct {
			Val int `xml:"val,attr"`
		} `xml:"buSzPct"`
	} `xml:"pPr"`
	DefRPr *drawingRPr `xml:"defRPr"`
	R      []struct {
		RPr *drawingRPr `xml:"rPr"`
		T   string      `xml:"t"`
	} `xml:"r"`
}

// drawingConnectionRef captures a connection-site reference on a connector.
type drawingConnectionRef struct {
	ID  int `xml:"id,attr"`
	Idx int `xml:"idx,attr"`
}

// drawingCxnSp captures a dsp:cxnSp connector shape.
type drawingCxnSp struct {
	NvCxnSpPr struct {
		CNvCxnSpPr struct {
			StCxn  *drawingConnectionRef `xml:"stCxn"`
			EndCxn *drawingConnectionRef `xml:"endCxn"`
		} `xml:"cNvCxnSpPr"`
	} `xml:"nvCxnSpPr"`
	SpPr drawingSpPr `xml:"spPr"`
}

// drawingGrpSp captures a dsp:grpSp group shape.
type drawingGrpSp struct {
	GrpSpPr struct {
		Xfrm drawingXfrm `xml:"xfrm"`
	} `xml:"grpSpPr"`
	Sp    []drawingSp    `xml:"sp"`
	CxnSp []drawingCxnSp `xml:"cxnSp"`
	GrpSp []drawingGrpSp `xml:"grpSp"`
}

// groupTransform is the combined transform from a group's local coordinate
// system to the diagram coordinate system.
type groupTransform struct {
	OffX, OffY  float64
	ScaleX      float64
	ScaleY      float64
	ChOffX      float64
	ChOffY      float64
	RotationDeg float64
	FlipH       bool
	FlipV       bool
}

// composeGroupTransforms returns a transform that maps from the child's local
// coordinate system to the diagram coordinate system.
func composeGroupTransforms(parent, child *groupTransform) *groupTransform {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	scaleX := parent.ScaleX * child.ScaleX
	scaleY := parent.ScaleY * child.ScaleY
	offX := parent.OffX + (child.OffX-parent.ChOffX)*parent.ScaleX
	offY := parent.OffY + (child.OffY-parent.ChOffY)*parent.ScaleY
	return &groupTransform{
		OffX:        offX,
		OffY:        offY,
		ScaleX:      scaleX,
		ScaleY:      scaleY,
		ChOffX:      child.ChOffX,
		ChOffY:      child.ChOffY,
		RotationDeg: parent.RotationDeg + child.RotationDeg,
		FlipH:       parent.FlipH != child.FlipH,
		FlipV:       parent.FlipV != child.FlipV,
	}
}

// groupTransformFromXfrm builds a groupTransform from a raw group xfrm.
func groupTransformFromXfrm(xfrm drawingXfrm) *groupTransform {
	gt := &groupTransform{
		OffX:        float64(xfrm.Off.X),
		OffY:        float64(xfrm.Off.Y),
		ScaleX:      1.0,
		ScaleY:      1.0,
		ChOffX:      float64(xfrm.ChOff.X),
		ChOffY:      float64(xfrm.ChOff.Y),
		RotationDeg: float64(xfrm.Rot) / 60000.0,
		FlipH:       xfrm.FlipH,
		FlipV:       xfrm.FlipV,
	}
	if xfrm.ChExt.CX != 0 {
		gt.ScaleX = float64(xfrm.Ext.CX) / float64(xfrm.ChExt.CX)
	}
	if xfrm.ChExt.CY != 0 {
		gt.ScaleY = float64(xfrm.Ext.CY) / float64(xfrm.ChExt.CY)
	}
	return gt
}

// applyGroupTransform composes the group transform onto a DrawingShapeInfo.
func applyGroupTransform(info *DrawingShapeInfo, gt *groupTransform) {
	if gt == nil {
		return
	}
	info.X = gt.OffX + (info.X-gt.ChOffX)*gt.ScaleX
	info.Y = gt.OffY + (info.Y-gt.ChOffY)*gt.ScaleY
	info.W *= gt.ScaleX
	info.H *= gt.ScaleY
	info.RotationDeg += gt.RotationDeg
	info.FlipH = info.FlipH != gt.FlipH
	info.FlipV = info.FlipV != gt.FlipV
}

// drawingProcessGroup recursively processes a group, applying composed
// transforms to all descendant shapes. It returns the first connector style
// found in the group. The order counter is incremented for every shape and
// connector encountered so the flattened list reflects the original spTree
// order.
func drawingProcessGroup(grp drawingGrpSp, parentGT *groupTransform, shapes map[string]*DrawingShapeInfo, connectors *[]*DrawingConnectorInfo, themeColors map[string]string, order *int) *drawing.LineStyle {
	gt := composeGroupTransforms(parentGT, groupTransformFromXfrm(grp.GrpSpPr.Xfrm))

	var connStyle *drawing.LineStyle
	for _, sp := range grp.Sp {
		if info := drawingShapeFromSp(sp, themeColors); info != nil {
			applyGroupTransform(info, gt)
			info.DrawingOrder = *order
			*order++
			shapes[info.ModelID] = info
		}
	}
	for _, cxn := range grp.CxnSp {
		if info := drawingConnectorFromCxnSp(cxn, themeColors); info != nil {
			applyGroupTransformToConnector(info, gt)
			info.DrawingOrder = *order
			*order++
			*connectors = append(*connectors, info)
			if connStyle == nil {
				connStyle = info.Line
			}
		}
	}
	for _, childGrp := range grp.GrpSp {
		if s := drawingProcessGroup(childGrp, gt, shapes, connectors, themeColors, order); s != nil && connStyle == nil {
			connStyle = s
		}
	}
	return connStyle
}

// drawingConnectorStyle resolves a connector line style from a dsp:cxnSp.
func drawingConnectorStyle(cxn drawingCxnSp, themeColors map[string]string) *drawing.LineStyle {
	return drawingResolveLineStyle(cxn.SpPr.Ln, themeColors)
}

// drawingResolveLineStyle converts a parsed drawingLn into a drawing.LineStyle.
func drawingResolveLineStyle(ln *drawingLn, themeColors map[string]string) *drawing.LineStyle {
	if ln == nil {
		return nil
	}
	line := &drawing.LineStyle{Width: 1.0}
	if ln.W > 0 {
		line.Width = float64(ln.W) / 12700.0
	}
	if ln.SolidFill.SrgbClr != nil {
		line.Color, line.Alpha = resolveDrawingColor(ln.SolidFill.SrgbClr)
	}
	if ln.SolidFill.SchemeClr != nil && themeColors != nil {
		ref := ln.SolidFill.SchemeClr.Val
		if color, ok := themeColors[ref]; ok {
			line.Color = color
			_, line.Alpha = resolveDrawingColor(ln.SolidFill.SchemeClr)
		}
	}
	if gf := drawingParseGradFill(ln.GradFill, themeColors); gf != nil {
		line.Fill = gf
		if grad, ok := gf.(drawing.GradientFill); ok && len(grad.Stops) > 0 {
			line.Color = grad.Stops[0].Color
			line.Alpha = grad.Stops[0].Alpha
		}
	}
	switch ln.Cap {
	case "round":
		line.Cap = drawing.LineCapRound
	case "square":
		line.Cap = drawing.LineCapSquare
	default:
		line.Cap = drawing.LineCapFlat
	}
	if ln.Cmpd != "" && ln.Cmpd != "sng" {
		line.Compound = true
		line.CompoundType = ln.Cmpd
	}
	if ln.PrstDash != nil && ln.PrstDash.Val != "" {
		line.Dash = drawing.DashPattern(ln.PrstDash.Val)
	}
	if ln.HeadEnd != nil {
		line.HeadEnd = &drawing.Arrowhead{
			Type:   drawing.ArrowheadType(ln.HeadEnd.Type),
			Size:   drawing.ArrowheadSize(ln.HeadEnd.W),
			Length: drawing.ArrowheadSize(ln.HeadEnd.Len),
		}
	}
	if ln.TailEnd != nil {
		line.TailEnd = &drawing.Arrowhead{
			Type:   drawing.ArrowheadType(ln.TailEnd.Type),
			Size:   drawing.ArrowheadSize(ln.TailEnd.W),
			Length: drawing.ArrowheadSize(ln.TailEnd.Len),
		}
	}
	if line.Color != "" || line.Fill != nil || ln.W > 0 || line.Cap != drawing.LineCapFlat || line.Compound || line.Dash != "" || line.HeadEnd != nil || line.TailEnd != nil {
		return line
	}
	return nil
}

// applyGroupTransformToConnector composes the group transform onto a
// DrawingConnectorInfo.
func applyGroupTransformToConnector(info *DrawingConnectorInfo, gt *groupTransform) {
	if gt == nil {
		return
	}
	info.X = gt.OffX + (info.X-gt.ChOffX)*gt.ScaleX
	info.Y = gt.OffY + (info.Y-gt.ChOffY)*gt.ScaleY
	info.W *= gt.ScaleX
	info.H *= gt.ScaleY
	info.RotationDeg += gt.RotationDeg
	info.FlipH = info.FlipH != gt.FlipH
	info.FlipV = info.FlipV != gt.FlipV
}

// drawingConnectorFromCxnSp converts a parsed dsp:cxnSp into a DrawingConnectorInfo.
func drawingConnectorFromCxnSp(cxn drawingCxnSp, themeColors map[string]string) *DrawingConnectorInfo {
	spPr := cxn.SpPr
	info := &DrawingConnectorInfo{
		X:           float64(spPr.Xfrm.Off.X),
		Y:           float64(spPr.Xfrm.Off.Y),
		W:           float64(spPr.Xfrm.Ext.CX),
		H:           float64(spPr.Xfrm.Ext.CY),
		RotationDeg: float64(spPr.Xfrm.Rot) / 60000.0,
		FlipH:       spPr.Xfrm.FlipH,
		FlipV:       spPr.Xfrm.FlipV,
		Geometry:    spPr.PrstGeom.Prst,
		Line:        drawingConnectorStyle(cxn, themeColors),
	}
	for _, gd := range spPr.PrstGeom.AvLst.Gd {
		if v, ok := parseAdjustmentFormula(gd.Fmla); ok {
			info.Adjustments = append(info.Adjustments, v)
		}
	}
	if st := cxn.NvCxnSpPr.CNvCxnSpPr.StCxn; st != nil {
		info.StCxnID = st.ID
		info.StCxnIdx = st.Idx
	}
	if en := cxn.NvCxnSpPr.CNvCxnSpPr.EndCxn; en != nil {
		info.EndCxnID = en.ID
		info.EndCxnIdx = en.Idx
	}
	return info
}

// drawingShapeFromSp converts a parsed dsp:sp into a DrawingShapeInfo.
func drawingShapeFromSp(sp drawingSp, themeColors map[string]string) *DrawingShapeInfo {
	if sp.ModelID == "" {
		return nil
	}

	info := &DrawingShapeInfo{
		ModelID:     sp.ModelID,
		SourceID:    sp.NvSpPr.CNvPr.ID,
		X:           float64(sp.SpPr.Xfrm.Off.X),
		Y:           float64(sp.SpPr.Xfrm.Off.Y),
		W:           float64(sp.SpPr.Xfrm.Ext.CX),
		H:           float64(sp.SpPr.Xfrm.Ext.CY),
		RotationDeg: float64(sp.SpPr.Xfrm.Rot) / 60000.0,
		FlipH:       sp.SpPr.Xfrm.FlipH,
		FlipV:       sp.SpPr.Xfrm.FlipV,
		Geometry:    sp.SpPr.PrstGeom.Prst,
	}
	for _, gd := range sp.SpPr.PrstGeom.AvLst.Gd {
		if v, ok := parseAdjustmentFormula(gd.Fmla); ok {
			info.Adjustments = append(info.Adjustments, v)
		}
	}
	if sp.SpPr.CustGeom != nil {
		if geom, _ := sp.SpPr.CustGeom.DrawingGeometryWithDiagnostics(); geom != nil {
			info.GeometryData = geom
		}
	}

	if sp.SpPr.SolidFill.SrgbClr != nil {
		color, alpha := resolveDrawingColor(sp.SpPr.SolidFill.SrgbClr)
		info.Fill = drawing.SolidFill{Color: color, Alpha: alpha}
	}
	if sp.SpPr.SolidFill.SchemeClr != nil && themeColors != nil {
		ref := sp.SpPr.SolidFill.SchemeClr.Val
		if color, ok := themeColors[ref]; ok {
			_, alpha := resolveDrawingColor(sp.SpPr.SolidFill.SchemeClr)
			info.Fill = drawing.SolidFill{Color: color, Alpha: alpha}
		}
	}
	if sp.SpPr.NoFill != nil {
		info.Fill = drawing.NoFill{}
	}
	if gf := drawingParseGradFill(sp.SpPr.GradFill, themeColors); gf != nil {
		info.Fill = gf
	}
	if sp.SpPr.PattFill != nil {
		p := sp.SpPr.PattFill
		fg, fgAlpha := "", 1.0
		if p.FgClr != nil {
			if p.FgClr.SrgbClr != nil {
				fg, fgAlpha = resolveDrawingColor(p.FgClr.SrgbClr)
			}
			if p.FgClr.SchemeClr != nil && themeColors != nil {
				if c, ok := themeColors[p.FgClr.SchemeClr.Val]; ok {
					fg = c
					_, fgAlpha = resolveDrawingColor(p.FgClr.SchemeClr)
				}
			}
		}
		bg, bgAlpha := "", 1.0
		if p.BgClr != nil {
			if p.BgClr.SrgbClr != nil {
				bg, bgAlpha = resolveDrawingColor(p.BgClr.SrgbClr)
			}
			if p.BgClr.SchemeClr != nil && themeColors != nil {
				if c, ok := themeColors[p.BgClr.SchemeClr.Val]; ok {
					bg = c
					_, bgAlpha = resolveDrawingColor(p.BgClr.SchemeClr)
				}
			}
		}
		if fg == "" {
			fg = "000000"
		}
		if bg == "" {
			bg = "FFFFFF"
		}
		info.Fill = drawing.PatternFill{
			PatternType: drawing.PatternType(p.Prst),
			Foreground:  fg,
			Background:  bg,
			ForeAlpha:   fgAlpha,
			BackAlpha:   bgAlpha,
		}
	}
	if sp.SpPr.BlipFill != nil && (sp.SpPr.BlipFill.Blip.Embed != "" || sp.SpPr.BlipFill.Blip.Link != "") {
		bf := sp.SpPr.BlipFill
		pf := &drawing.PictureFill{
			EmbedID: bf.Blip.Embed,
			Alpha:   1.0,
			DPI:     bf.DPI,
		}
		if bf.RotWithShape != "" {
			pf.RotWithShapeSet = true
			pf.RotWithShape = bf.RotWithShape != "0" && bf.RotWithShape != "false"
		}
		if bf.Tile != nil {
			pf.Tile = true
			pf.TileOffsetX = bf.Tile.Tx
			pf.TileOffsetY = bf.Tile.Ty
			pf.TileScaleX = bf.Tile.Sx
			pf.TileScaleY = bf.Tile.Sy
			pf.TileFlip = bf.Tile.Flip
			pf.TileAlign = bf.Tile.Algn
		} else {
			pf.Stretch = true
		}
		if bf.Stretch != nil && bf.Stretch.FillRect != nil {
			fr := bf.Stretch.FillRect
			left := clampPictureFraction(fr.L)
			top := clampPictureFraction(fr.T)
			right := clampPictureFraction(fr.R)
			bottom := clampPictureFraction(fr.B)
			pf.FillRect = &drawing.Rect{X: left, Y: top, Width: maxPictureFraction(0, 1-left-right), Height: maxPictureFraction(0, 1-top-bottom)}
		}
		if sr := bf.SrcRect; sr != nil {
			left := clampPictureFraction(sr.L)
			top := clampPictureFraction(sr.T)
			right := clampPictureFraction(sr.R)
			bottom := clampPictureFraction(sr.B)
			pf.Crop = &drawing.Rect{
				X:      left,
				Y:      top,
				Width:  maxPictureFraction(0, 1-left-right),
				Height: maxPictureFraction(0, 1-top-bottom),
			}
		}
		var effects []drawing.Effect
		if bf.Blip.Grayscl != nil {
			effects = append(effects, drawing.Grayscale{})
		}
		if bf.Blip.BiLevel != nil {
			effects = append(effects, drawing.BiLevel{Threshold: bf.Blip.BiLevel.Thresh})
		}
		if dt := bf.Blip.Duotone; dt != nil {
			effects = append(effects, drawing.Duotone{
				Color1: drawingColorRefString(&dt.Clr1, themeColors),
				Color2: drawingColorRefString(&dt.Clr2, themeColors),
			})
		}
		if lum := bf.Blip.Lum; lum != nil {
			effects = append(effects, drawing.Lum{Brightness: lum.Bright, Contrast: lum.Contrast})
		}
		if am := bf.Blip.AlphaMod; am != nil {
			pf.Alpha = float64(am.Amt) / 100000.0
			effects = append(effects, drawing.AlphaMod{Amount: am.Amt})
		}
		if amf := bf.Blip.AlphaModFix; amf != nil {
			effects = append(effects, drawing.AlphaModFix{Amount: amf.Amt})
		}
		if ao := bf.Blip.AlphaOff; ao != nil {
			effects = append(effects, drawing.AlphaOff{Amount: ao.Amt})
		}
		if ar := bf.Blip.AlphaRepl; ar != nil {
			effects = append(effects, drawing.AlphaRepl{Alpha: ar.A})
		}
		if ab := bf.Blip.AlphaBiLevel; ab != nil {
			effects = append(effects, drawing.AlphaBiLevel{Threshold: ab.Thresh})
		}
		if ao := bf.Blip.AlphaOutset; ao != nil {
			effects = append(effects, drawing.AlphaOutset{Radius: drawing.EMUToPoints(ao.Rad)})
		}
		if bf.Blip.AlphaInv != nil {
			effects = append(effects, drawing.AlphaInv{})
		}
		if bf.Blip.AlphaCeiling != nil {
			effects = append(effects, drawing.AlphaCeiling{})
		}
		if bf.Blip.AlphaFloor != nil {
			effects = append(effects, drawing.AlphaFloor{})
		}
		if hsl := bf.Blip.HSL; hsl != nil {
			effects = append(effects, drawing.HSLEffect{Hue: hsl.Hue, Sat: hsl.Sat, Lum: hsl.Lum})
		}
		if cr := bf.Blip.ClrRepl; cr != nil {
			effects = append(effects, drawing.ClrRepl{Color: drawingColorRefString(cr, themeColors)})
		}
		if cc := bf.Blip.ClrChange; cc != nil {
			effects = append(effects, drawing.ClrChange{From: drawingColorRefString(&cc.From, themeColors), To: drawingColorRefString(&cc.To, themeColors), FromAlpha: 1, ToAlpha: 1, UseAlpha: cc.UseAlpha})
		}
		pf.BlipEffects = effects
		info.Fill = pf
	}
	if sp.SpPr.Ln != nil {
		info.Line = drawingResolveLineStyle(sp.SpPr.Ln, themeColors)
	}

	info.Effect = drawingEffectListToEffects(sp.SpPr.EffectLst, themeColors)
	if threeD := drawingThreeDEffect(sp.SpPr.Scene3D, sp.SpPr.Sp3D, themeColors); threeD != nil {
		info.Effect.Effects = append(info.Effect.Effects, *threeD)
	}

	var paragraphs []*smartart.TextParagraph
	var texts []string
	for _, p := range sp.TxBody.P {
		para := &smartart.TextParagraph{}
		if p.PPr != nil {
			para.Align = p.PPr.Algn
			para.Level = p.PPr.Lvl
			para.DefaultTabSizeEMU = p.PPr.DefTabSz
			if len(p.PPr.TabStops) > 0 {
				para.TabStops = make([]smartart.TabStop, 0, len(p.PPr.TabStops))
				for _, stop := range p.PPr.TabStops {
					para.TabStops = append(para.TabStops, smartart.TabStop{Pos: stop.Pos, Algn: stop.Algn})
				}
			}
			if p.PPr.BuChar != nil {
				para.BulletChar = p.PPr.BuChar.Char
			}
			if p.PPr.BuAutoNum != nil {
				para.AutoNum = true
				para.AutoNumType = p.PPr.BuAutoNum.Type
				para.AutoNumStart = p.PPr.BuAutoNum.StartAt
			}
			if p.PPr.BuClr != nil {
				para.BulletColor, _ = resolveDrawingColor(p.PPr.BuClr)
			}
			if p.PPr.BuClrScheme != nil && themeColors != nil {
				ref := p.PPr.BuClrScheme.Val
				if color, ok := themeColors[ref]; ok {
					para.BulletColor = color
				}
			}
			if p.PPr.BuFont != nil {
				para.BulletFontFamily = p.PPr.BuFont.Typeface
			}
			if p.PPr.BuSzPts != nil {
				para.BulletSize = p.PPr.BuSzPts.Val
			}
			if p.PPr.BuSzPct != nil {
				para.BulletSizePct = p.PPr.BuSzPct.Val
			}
			if p.PPr.BuNone != nil {
				para.BulletChar = ""
				para.AutoNum = false
			}
			if p.PPr.SpcBef != nil {
				if p.PPr.SpcBef.SpcPts != nil {
					para.SpaceBefore = p.PPr.SpcBef.SpcPts.Val
				}
				if p.PPr.SpcBef.SpcPct != nil {
					para.SpaceBeforePct = p.PPr.SpcBef.SpcPct.Val
				}
			}
			if p.PPr.SpcAft != nil {
				if p.PPr.SpcAft.SpcPts != nil {
					para.SpaceAfter = p.PPr.SpcAft.SpcPts.Val
				}
				if p.PPr.SpcAft.SpcPct != nil {
					para.SpaceAfterPct = p.PPr.SpcAft.SpcPct.Val
				}
			}
			if p.PPr.LnSpc != nil {
				if p.PPr.LnSpc.SpcPts != nil {
					para.LineSpacingPts = p.PPr.LnSpc.SpcPts.Val
				}
				if p.PPr.LnSpc.SpcPct != nil {
					para.LineSpacingPct = p.PPr.LnSpc.SpcPct.Val
				}
			}
		}
		for _, r := range p.R {
			if r.T != "" {
				texts = append(texts, r.T)
			}
			run := smartart.TextRun{Text: r.T}
			rpr := r.RPr
			if rpr == nil {
				rpr = p.DefRPr
			}
			if rpr != nil {
				run.Bold = rpr.B == "1"
				if rpr.Latin != nil {
					run.FontFamily = rpr.Latin.Typeface
				}
				if rpr.EA != nil {
					run.FontFamilyEA = rpr.EA.Typeface
				}
				if rpr.CS != nil {
					run.FontFamilyCS = rpr.CS.Typeface
				}
				run.Italic = rpr.I == "1"
				run.Underline = rpr.U != ""
				run.FontSize = int(rpr.Sz)
				if rpr.SldFill.SrgbClr != nil {
					run.Color, _ = resolveDrawingColor(rpr.SldFill.SrgbClr)
				}
				if rpr.SldFill.SchemeClr != nil && themeColors != nil {
					ref := rpr.SldFill.SchemeClr.Val
					if color, ok := themeColors[ref]; ok {
						run.Color = color
					}
				}
				if rpr.EffectLst != nil {
					run.Effects = smartart.ParseEffectListInnerXMLWithTheme(rpr.EffectLst.InnerXML, themeColors)
				}
			}
			para.Runs = append(para.Runs, run)
		}
		if len(para.Runs) > 0 {
			paragraphs = append(paragraphs, para)
		}
	}
	info.Paragraphs = paragraphs
	info.Text = strings.Join(texts, "")
	info.TextBodyProps = bodyPrToProperties(sp.TxBody.BodyPr)

	return info
}

// resolveDrawingColor converts a drawingColorInfo to a hex color and alpha.
func resolveDrawingColor(c *drawingColorInfo) (string, float64) {
	if c == nil {
		return "", 1.0
	}
	alpha := 1.0
	if c.Alpha != nil {
		alpha = float64(c.Alpha.Val) / 100000.0
	}
	if c.AlphaMod != nil {
		alpha *= float64(c.AlphaMod.Val) / 100000.0
	}
	if c.AlphaOff != nil {
		alpha += float64(c.AlphaOff.Val) / 100000.0
	}
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}
	return c.Val, alpha
}

// drawingEffectListToEffects converts a parsed drawing effect list into shared
// drawing.Effect values.  It resolves scheme colors through themeColors.
func drawingEffectListToEffects(el *drawingEffectList, themeColors map[string]string) drawing.EffectList {
	if el == nil {
		return drawing.EffectList{}
	}
	if strings.TrimSpace(el.InnerXML) != "" {
		return smartart.ParseEffectListInnerXMLWithTheme(el.InnerXML, themeColors)
	}
	var effects []drawing.Effect
	resolveColor := func(srgb, scheme *drawingColorInfo) (string, float64) {
		if srgb != nil {
			return resolveDrawingColor(srgb)
		}
		if scheme != nil {
			color := "000000"
			if themeColors != nil {
				if c, ok := themeColors[scheme.Val]; ok {
					color = c
				}
			}
			_, alpha := resolveDrawingColor(scheme)
			return color, alpha
		}
		return "000000", 1.0
	}
	if el.OuterShadow != nil {
		color, alpha := resolveColor(el.OuterShadow.SrgbClr, el.OuterShadow.SchemeClr)
		effects = append(effects, drawing.Shadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(el.OuterShadow.BlurRad),
			Distance: drawing.EMUToPoints(el.OuterShadow.Dist),
			Angle:    float64(el.OuterShadow.Dir) / 60000.0,
			ScaleX:   1.0,
			ScaleY:   1.0,
		})
	}
	if el.InnerShadow != nil {
		color, alpha := resolveColor(el.InnerShadow.SrgbClr, el.InnerShadow.SchemeClr)
		effects = append(effects, drawing.InnerShadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(el.InnerShadow.BlurRad),
			Distance: drawing.EMUToPoints(el.InnerShadow.Dist),
			Angle:    float64(el.InnerShadow.Dir) / 60000.0,
		})
	}
	if el.Glow != nil {
		color, alpha := resolveColor(el.Glow.SrgbClr, el.Glow.SchemeClr)
		effects = append(effects, drawing.Glow{
			Color:  color,
			Alpha:  alpha,
			Radius: drawing.EMUToPoints(el.Glow.Rad),
		})
	}
	if el.SoftEdge != nil {
		effects = append(effects, drawing.SoftEdge{
			Radius: drawing.EMUToPoints(el.SoftEdge.Rad),
		})
	}
	if el.Reflection != nil {
		alpha := 0.3
		if el.Reflection.StA > 0 {
			alpha = float64(el.Reflection.StA) / 100000.0
		}
		effects = append(effects, drawing.Reflection{
			Alpha:     alpha,
			Blur:      drawing.EMUToPoints(el.Reflection.BlurRad),
			Distance:  drawing.EMUToPoints(el.Reflection.Dist),
			Direction: float64(el.Reflection.Dir) / 60000.0,
			Size:      1.0,
			Fade:      true,
		})
	}
	if el.Blur != nil {
		effects = append(effects, drawing.Blur{
			Radius: drawing.EMUToPoints(el.Blur.Rad),
		})
	}
	return drawing.EffectList{Effects: effects}
}

// drawingThreeDEffect converts parsed scene3d/sp3d properties into a shared
// drawing.ThreeD approximation.
func drawingThreeDEffect(scene *drawingScene3D, sp3d *drawingSp3D, themeColors map[string]string) *drawing.ThreeD {
	if scene == nil && sp3d == nil {
		return nil
	}
	effect := &drawing.ThreeD{}
	if scene != nil {
		if scene.Camera != nil {
			effect.CameraPreset = scene.Camera.Prst
			if scene.Camera.FOV != 0 {
				effect.CameraFOV = float64(scene.Camera.FOV) / 60000.0
			}
			if scene.Camera.Zoom != 0 {
				effect.CameraZoom = float64(scene.Camera.Zoom) / 100000.0
			}
			if scene.Camera.Rot != nil {
				effect.RotationX = float64(scene.Camera.Rot.Lat) / 60000.0
				effect.RotationY = float64(scene.Camera.Rot.Lon) / 60000.0
				effect.RotationZ = float64(scene.Camera.Rot.Rev) / 60000.0
			}
		}
		if scene.LightRig != nil {
			effect.LightRig = scene.LightRig.Rig
			effect.LightDirection = scene.LightRig.Dir
			if scene.LightRig.Rot != nil {
				effect.LightRotationX = float64(scene.LightRig.Rot.Lat) / 60000.0
				effect.LightRotationY = float64(scene.LightRig.Rot.Lon) / 60000.0
				effect.LightRotationZ = float64(scene.LightRig.Rot.Rev) / 60000.0
			}
		}
	}
	if sp3d != nil {
		effect.Material = sp3d.PrstMaterial
		if sp3d.ExtrusionH != 0 || sp3d.Z != 0 {
			if sp3d.ExtrusionH > sp3d.Z {
				effect.ExtrusionDepth = drawing.EMUToPoints(sp3d.ExtrusionH)
			} else {
				effect.ExtrusionDepth = drawing.EMUToPoints(sp3d.Z)
			}
		}
		effect.ContourWidth = drawing.EMUToPoints(sp3d.ContourW)
		if sp3d.BevelT != nil {
			effect.BevelTopWidth = drawing.EMUToPoints(sp3d.BevelT.W)
			effect.BevelTopHeight = drawing.EMUToPoints(sp3d.BevelT.H)
			effect.BevelTopPreset = sp3d.BevelT.Prst
		}
		if sp3d.BevelB != nil {
			effect.BevelBottomWidth = drawing.EMUToPoints(sp3d.BevelB.W)
			effect.BevelBottomHeight = drawing.EMUToPoints(sp3d.BevelB.H)
			effect.BevelBottomPreset = sp3d.BevelB.Prst
		}
		resolve3DColor := func(srgb, scheme *drawingColorInfo) string {
			if srgb != nil && srgb.Val != "" {
				return srgb.Val
			}
			if scheme != nil && themeColors != nil {
				if c := themeColors[scheme.Val]; c != "" {
					return c
				}
			}
			return ""
		}
		effect.ContourColor = resolve3DColor(sp3d.ContourClr, sp3d.ContourScheme)
		effect.ExtrusionColor = resolve3DColor(sp3d.ExtrusionClr, sp3d.ExtrudeScheme)
	}
	if effect.CameraPreset == "" && effect.LightRig == "" && effect.LightDirection == "" &&
		effect.Material == "" && effect.RotationX == 0 && effect.RotationY == 0 && effect.RotationZ == 0 &&
		effect.ExtrusionDepth == 0 && effect.ContourWidth == 0 && effect.BevelTopHeight == 0 && effect.BevelBottomHeight == 0 {
		return nil
	}
	return effect
}

// drawingParseGradFill converts a drawingGradFill to a drawing.GradientFill.
func drawingParseGradFill(gf *drawingGradFill, themeColors map[string]string) drawing.Fill {
	if gf == nil || len(gf.GsLst) == 0 {
		return nil
	}
	var stops []drawing.GradientStop
	for _, gs := range gf.GsLst {
		var color string
		var alpha float64 = 1.0
		if gs.SrgbClr != nil {
			color, alpha = resolveDrawingColor(gs.SrgbClr)
		}
		if gs.SchemeClr != nil && themeColors != nil {
			if c, ok := themeColors[gs.SchemeClr.Val]; ok {
				color = c
				_, alpha = resolveDrawingColor(gs.SchemeClr)
			}
		}
		if color == "" {
			continue
		}
		stops = append(stops, drawing.GradientStop{
			Position: float64(gs.Pos) / 100000.0,
			Color:    color,
			Alpha:    alpha,
		})
	}
	if len(stops) == 0 {
		return nil
	}
	angle := 0.0
	if gf.Lin != nil {
		angle = float64(gf.Lin.Ang) / 60000.0
	}
	return drawing.GradientFill{
		GradientType: drawing.GradientTypeLinear,
		Angle:        angle,
		Stops:        stops,
	}
}

// parseAdjustmentFormula converts an a:gd formula such as "val 50000" into a
// normalized 0-1 adjustment value. Only literal "val N" formulas are supported;
// other formula types return false.
func parseAdjustmentFormula(fmla string) (float64, bool) {
	parts := strings.Fields(fmla)
	if len(parts) != 2 || parts[0] != "val" {
		return 0, false
	}
	v, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return float64(v) / 100000.0, true
}

// DrawingBounds computes the bounding box of a collection of drawing shapes.
func DrawingBounds(shapes map[string]*DrawingShapeInfo) (minX, minY, maxX, maxY float64, ok bool) {
	first := true
	for _, s := range shapes {
		if first {
			minX, minY = s.X, s.Y
			maxX, maxY = s.X+s.W, s.Y+s.H
			first = false
			continue
		}
		if s.X < minX {
			minX = s.X
		}
		if s.Y < minY {
			minY = s.Y
		}
		if s.X+s.W > maxX {
			maxX = s.X + s.W
		}
		if s.Y+s.H > maxY {
			maxY = s.Y + s.H
		}
	}
	return minX, minY, maxX, maxY, !first
}

// ExtractConnectorStyle returns the connector line style defined by a
// pre-rendered dsp:drawing part, or nil if the drawing contains no connector
// shape.
func ExtractConnectorStyle(drawingXML []byte, themeColors map[string]string) *drawing.LineStyle {
	_, connStyle, _ := parseDiagramDrawing(drawingXML, themeColors)
	return connStyle
}

// ExtractConnectors returns the pre-rendered connector geometries from a
// dsp:drawing XML part.  Connectors are returned in the order they appear in
// the source spTree.
func ExtractConnectors(drawingXML []byte, themeColors map[string]string) []*DrawingConnectorInfo {
	_, connectors, _, _ := parseDiagramDrawingWithConnectors(drawingXML, themeColors)
	return connectors
}

// ExtractShapeInfos returns the pre-rendered shape geometries from a
// dsp:drawing XML part, keyed by their modelId attribute.
func ExtractShapeInfos(drawingXML []byte, themeColors map[string]string) map[string]*DrawingShapeInfo {
	shapes, _, _ := parseDiagramDrawing(drawingXML, themeColors)
	return shapes
}
