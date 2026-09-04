// Package drawingxml provides shared XML-facing types for DrawingML elements.
// These types are used for parsing XML from Word, Excel, and PowerPoint files,
// and can be converted to the normalized drawing package types.
package drawingxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
)

// SolidFill represents a:solidFill or wps:solidFill.
type SolidFill struct {
	XMLName   xml.Name   `xml:"solidFill"`
	SRGBClr   *SRGBClr   `xml:"srgbClr"`
	SchemeClr *SchemeClr `xml:"schemeClr"`
	SysClr    *SysClr    `xml:"sysClr"`
	PrstClr   *PrstClr   `xml:"prstClr"`
	ScrGBClr  *ScrGBClr  `xml:"scrgbClr"`
	HSLClr    *HSLClr    `xml:"hslClr"`
}

type colorTransforms struct {
	Alpha    *ColorTransform `xml:"alpha"`
	AlphaMod *ColorTransform `xml:"alphaMod"`
	AlphaOff *ColorTransform `xml:"alphaOff"`
	Shade    *ColorTransform `xml:"shade"`
	Tint     *ColorTransform `xml:"tint"`
	Red      *ColorTransform `xml:"red"`
	RedMod   *ColorTransform `xml:"redMod"`
	RedOff   *ColorTransform `xml:"redOff"`
	Green    *ColorTransform `xml:"green"`
	GreenMod *ColorTransform `xml:"greenMod"`
	GreenOff *ColorTransform `xml:"greenOff"`
	Blue     *ColorTransform `xml:"blue"`
	BlueMod  *ColorTransform `xml:"blueMod"`
	BlueOff  *ColorTransform `xml:"blueOff"`
	Hue      *ColorTransform `xml:"hue"`
	Sat      *ColorTransform `xml:"sat"`
	Lum      *ColorTransform `xml:"lum"`
	LumMod   *ColorTransform `xml:"lumMod"`
	LumOff   *ColorTransform `xml:"lumOff"`
	SatMod   *ColorTransform `xml:"satMod"`
	SatOff   *ColorTransform `xml:"satOff"`
	HueMod   *ColorTransform `xml:"hueMod"`
	HueOff   *ColorTransform `xml:"hueOff"`
	Gray     *struct{}       `xml:"gray"`
	Comp     *struct{}       `xml:"comp"`
	Inv      *struct{}       `xml:"inv"`
	Gamma    *struct{}       `xml:"gamma"`
	InvGamma *struct{}       `xml:"invGamma"`
}

// SRGBClr represents a:srgbClr — an sRGB colour literal.
type SRGBClr struct {
	XMLName  xml.Name        `xml:"srgbClr"`
	Val      string          `xml:"val,attr"`
	Alpha    *ColorTransform `xml:"alpha"`
	AlphaMod *ColorTransform `xml:"alphaMod"`
	AlphaOff *ColorTransform `xml:"alphaOff"`
	Shade    *ColorTransform `xml:"shade"`
	Tint     *ColorTransform `xml:"tint"`
	Red      *ColorTransform `xml:"red"`
	RedMod   *ColorTransform `xml:"redMod"`
	RedOff   *ColorTransform `xml:"redOff"`
	Green    *ColorTransform `xml:"green"`
	GreenMod *ColorTransform `xml:"greenMod"`
	GreenOff *ColorTransform `xml:"greenOff"`
	Blue     *ColorTransform `xml:"blue"`
	BlueMod  *ColorTransform `xml:"blueMod"`
	BlueOff  *ColorTransform `xml:"blueOff"`
	Hue      *ColorTransform `xml:"hue"`
	Sat      *ColorTransform `xml:"sat"`
	Lum      *ColorTransform `xml:"lum"`
	LumMod   *ColorTransform `xml:"lumMod"`
	LumOff   *ColorTransform `xml:"lumOff"`
	SatMod   *ColorTransform `xml:"satMod"`
	SatOff   *ColorTransform `xml:"satOff"`
	HueMod   *ColorTransform `xml:"hueMod"`
	HueOff   *ColorTransform `xml:"hueOff"`
	Gray     *struct{}       `xml:"gray"`
	Comp     *struct{}       `xml:"comp"`
	Inv      *struct{}       `xml:"inv"`
	Gamma    *struct{}       `xml:"gamma"`
	InvGamma *struct{}       `xml:"invGamma"`
}

// SchemeClr represents a:schemeClr — a theme colour reference.
type SchemeClr struct {
	XMLName  xml.Name        `xml:"schemeClr"`
	Val      string          `xml:"val,attr"`
	Alpha    *ColorTransform `xml:"alpha"`
	AlphaMod *ColorTransform `xml:"alphaMod"`
	AlphaOff *ColorTransform `xml:"alphaOff"`
	Shade    *ColorTransform `xml:"shade"`
	Tint     *ColorTransform `xml:"tint"`
	Red      *ColorTransform `xml:"red"`
	RedMod   *ColorTransform `xml:"redMod"`
	RedOff   *ColorTransform `xml:"redOff"`
	Green    *ColorTransform `xml:"green"`
	GreenMod *ColorTransform `xml:"greenMod"`
	GreenOff *ColorTransform `xml:"greenOff"`
	Blue     *ColorTransform `xml:"blue"`
	BlueMod  *ColorTransform `xml:"blueMod"`
	BlueOff  *ColorTransform `xml:"blueOff"`
	Hue      *ColorTransform `xml:"hue"`
	Sat      *ColorTransform `xml:"sat"`
	Lum      *ColorTransform `xml:"lum"`
	LumMod   *ColorTransform `xml:"lumMod"`
	LumOff   *ColorTransform `xml:"lumOff"`
	SatMod   *ColorTransform `xml:"satMod"`
	SatOff   *ColorTransform `xml:"satOff"`
	HueMod   *ColorTransform `xml:"hueMod"`
	HueOff   *ColorTransform `xml:"hueOff"`
	Gray     *struct{}       `xml:"gray"`
	Comp     *struct{}       `xml:"comp"`
	Inv      *struct{}       `xml:"inv"`
	Gamma    *struct{}       `xml:"gamma"`
	InvGamma *struct{}       `xml:"invGamma"`
}

// SysClr represents a:sysClr — a system colour with last used RGB fallback.
type SysClr struct {
	XMLName xml.Name `xml:"sysClr"`
	Val     string   `xml:"val,attr"`
	LastClr string   `xml:"lastClr,attr"`
	colorTransforms
}

// PrstClr represents a:prstClr — a preset named colour.
type PrstClr struct {
	XMLName xml.Name `xml:"prstClr"`
	Val     string   `xml:"val,attr"`
	colorTransforms
}

// ScrGBClr represents a:scrgbClr — linear percentage RGB channels.
type ScrGBClr struct {
	XMLName xml.Name `xml:"scrgbClr"`
	R       int      `xml:"r,attr"`
	G       int      `xml:"g,attr"`
	B       int      `xml:"b,attr"`
	colorTransforms
}

// HSLClr represents a:hslClr — HSL color source.
type HSLClr struct {
	XMLName xml.Name `xml:"hslClr"`
	Hue     int      `xml:"hue,attr"`
	Sat     int      `xml:"sat,attr"`
	Lum     int      `xml:"lum,attr"`
	colorTransforms
}

// ColorTransform represents a DrawingML color-transform value such as
// <a:lumMod val="95000"/> or <a:lumOff val="5000"/>.
type ColorTransform struct {
	XMLName xml.Name `xml:""` // Will be set by UnmarshalXML
	Val     int      `xml:"val,attr"`
}

// UnmarshalXML handles different color transform elements (lumMod, lumOff, etc.)
func (c *ColorTransform) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	c.XMLName = start.Name
	for _, attr := range start.Attr {
		if attr.Name.Local == "val" {
			if val, err := strconv.Atoi(attr.Value); err == nil {
				c.Val = val
			}
			break
		}
	}
	// Skip to end element
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if _, ok := token.(xml.EndElement); ok {
			break
		}
	}
	return nil
}

// Line represents a:ln — line/border properties.
// LineEnd describes an arrowhead or endpoint decoration on a line.
type LineEnd struct {
	XMLName xml.Name `xml:"headEnd"`
	Type    string   `xml:"type,attr,omitempty"` // "none","triangle","arrow","stealth","diamond","oval"
	W       string   `xml:"w,attr,omitempty"`    // "sm","med","lg"
	Len     string   `xml:"len,attr,omitempty"`  // "sm","med","lg"
}

// LineTailEnd describes the tail arrowhead on a line (same structure as headEnd).
type LineTailEnd struct {
	XMLName xml.Name `xml:"tailEnd"`
	Type    string   `xml:"type,attr,omitempty"` // "none","triangle","arrow","stealth","diamond","oval"
	W       string   `xml:"w,attr,omitempty"`    // "sm","med","lg"
	Len     string   `xml:"len,attr,omitempty"`  // "sm","med","lg"
}

// PresetDash stores a preset dash pattern for a line.
type PresetDash struct {
	XMLName xml.Name `xml:"prstDash"`
	Val     string   `xml:"val,attr,omitempty"` // "solid","dot","dash","lgDash","dashDot","lgDashDot","lgDashDotDot"
}

// LineMiter stores an optional miter limit on a line join.
type LineMiter struct {
	XMLName xml.Name `xml:"miter"`
	Lim     int      `xml:"lim,attr,omitempty"`
}

// LineRound stores a round line join marker.
type LineRound struct {
	XMLName xml.Name `xml:"round"`
}

// LineBevel stores a bevel line join marker.
type LineBevel struct {
	XMLName xml.Name `xml:"bevel"`
}

// CustomDash stores a custom dash pattern for a line.
type CustomDash struct {
	XMLName xml.Name   `xml:"custDash"`
	Ds      []DashStop `xml:"ds"`
}

// DashStop stores one segment of a custom dash pattern.
type DashStop struct {
	XMLName xml.Name `xml:"ds"`
	D       int      `xml:"d,attr,omitempty"`
	Sp      int      `xml:"sp,attr,omitempty"`
}

type Line struct {
	XMLName   xml.Name      `xml:"ln"`
	W         int64         `xml:"w,attr,omitempty"`    // width in EMUs (12700 EMU = 1 pt)
	Cap       string        `xml:"cap,attr,omitempty"`  // line cap: "rnd", "sq", "flat"
	Cmpd      string        `xml:"cmpd,attr,omitempty"` // compound type: "sng", "dbl", "thickThin", "thinThick", "tri"
	SolidFill *SolidFill    `xml:"solidFill"`
	NoFill    *struct{}     `xml:"noFill"`
	GradFill  *GradientFill `xml:"gradFill"`
	PattFill  *PatternFill  `xml:"pattFill"`
	PrstDash  *PresetDash   `xml:"prstDash"`
	CustDash  *CustomDash   `xml:"custDash"`
	HeadEnd   *LineEnd      `xml:"headEnd"`
	TailEnd   *LineTailEnd  `xml:"tailEnd"`
	Round     *LineRound    `xml:"round"`
	Miter     *LineMiter    `xml:"miter"`
	Bevel     *LineBevel    `xml:"bevel"`
}

// ToDrawingSolidFill converts a SolidFill to a drawing.SolidFill.
func (sf *SolidFill) ToDrawingSolidFill() *drawing.SolidFill {
	if sf == nil {
		return nil
	}
	switch {
	case sf.SRGBClr != nil:
		return sf.SRGBClr.toDrawingSolidFill()
	case sf.SchemeClr != nil:
		return sf.SchemeClr.toDrawingSolidFill()
	case sf.SysClr != nil:
		return sf.SysClr.toDrawingSolidFill()
	case sf.PrstClr != nil:
		return sf.PrstClr.toDrawingSolidFill()
	case sf.ScrGBClr != nil:
		return sf.ScrGBClr.toDrawingSolidFill()
	case sf.HSLClr != nil:
		return sf.HSLClr.toDrawingSolidFill()
	}
	return nil
}

// ResolveSolidFill converts a SolidFill to a concrete drawing.SolidFill and
// resolves any theme-color references against the provided theme color map.
func ResolveSolidFill(sf *SolidFill, themeColors map[string]string) *drawing.SolidFill {
	if sf == nil {
		return nil
	}
	return resolveDrawingSolidFill(sf.ToDrawingSolidFill(), themeColors)
}

func applySRGBTransforms(fill *drawing.SolidFill, color *SRGBClr) {
	if fill == nil || color == nil {
		return
	}
	applyColorTransforms(fill, color.Alpha, color.AlphaMod, color.AlphaOff, color.Shade, color.Tint, color.Red, color.RedMod, color.RedOff, color.Green, color.GreenMod, color.GreenOff, color.Blue, color.BlueMod, color.BlueOff, color.Hue, color.Sat, color.Lum, color.LumMod, color.LumOff, color.SatMod, color.SatOff, color.HueMod, color.HueOff, color.Gray != nil, color.Comp != nil, color.Inv != nil, color.Gamma != nil, color.InvGamma != nil)
}

func applySchemeTransforms(fill *drawing.SolidFill, color *SchemeClr) {
	if fill == nil || color == nil {
		return
	}
	applyColorTransforms(fill, color.Alpha, color.AlphaMod, color.AlphaOff, color.Shade, color.Tint, color.Red, color.RedMod, color.RedOff, color.Green, color.GreenMod, color.GreenOff, color.Blue, color.BlueMod, color.BlueOff, color.Hue, color.Sat, color.Lum, color.LumMod, color.LumOff, color.SatMod, color.SatOff, color.HueMod, color.HueOff, color.Gray != nil, color.Comp != nil, color.Inv != nil, color.Gamma != nil, color.InvGamma != nil)
}

func (ct colorTransforms) apply(fill *drawing.SolidFill) {
	applyColorTransforms(fill, ct.Alpha, ct.AlphaMod, ct.AlphaOff, ct.Shade, ct.Tint, ct.Red, ct.RedMod, ct.RedOff, ct.Green, ct.GreenMod, ct.GreenOff, ct.Blue, ct.BlueMod, ct.BlueOff, ct.Hue, ct.Sat, ct.Lum, ct.LumMod, ct.LumOff, ct.SatMod, ct.SatOff, ct.HueMod, ct.HueOff, ct.Gray != nil, ct.Comp != nil, ct.Inv != nil, ct.Gamma != nil, ct.InvGamma != nil)
}

func (c *SRGBClr) toDrawingSolidFill() *drawing.SolidFill {
	if c == nil {
		return nil
	}
	fill := &drawing.SolidFill{Alpha: 1.0, Color: c.Val}
	applySRGBTransforms(fill, c)
	return fill
}

func (c *SchemeClr) toDrawingSolidFill() *drawing.SolidFill {
	if c == nil {
		return nil
	}
	fill := &drawing.SolidFill{Alpha: 1.0, ThemeColor: c.Val}
	applySchemeTransforms(fill, c)
	return fill
}

func (c *SysClr) toDrawingSolidFill() *drawing.SolidFill {
	if c == nil {
		return nil
	}
	color := strings.TrimSpace(c.LastClr)
	if color == "" {
		color = systemColorHex(c.Val)
	}
	fill := &drawing.SolidFill{Alpha: 1.0, Color: color}
	c.colorTransforms.apply(fill)
	return fill
}

func (c *PrstClr) toDrawingSolidFill() *drawing.SolidFill {
	if c == nil {
		return nil
	}
	fill := &drawing.SolidFill{Alpha: 1.0, Color: presetColorHex(c.Val)}
	c.colorTransforms.apply(fill)
	return fill
}

func (c *ScrGBClr) toDrawingSolidFill() *drawing.SolidFill {
	if c == nil {
		return nil
	}
	fill := &drawing.SolidFill{Alpha: 1.0, Color: formatHexRGBFromPercent(c.R, c.G, c.B)}
	c.colorTransforms.apply(fill)
	return fill
}

func (c *HSLClr) toDrawingSolidFill() *drawing.SolidFill {
	if c == nil {
		return nil
	}
	fill := &drawing.SolidFill{Alpha: 1.0, Color: formatHexRGBFromHSL(c.Hue, c.Sat, c.Lum)}
	c.colorTransforms.apply(fill)
	return fill
}

func applyColorTransforms(fill *drawing.SolidFill, alpha, alphaMod, alphaOff, shade, tint, red, redMod, redOff, green, greenMod, greenOff, blue, blueMod, blueOff, hue, sat, lum, lumMod, lumOff, satMod, satOff, hueMod, hueOff *ColorTransform, gray, comp, inv, gamma, invGamma bool) {
	if fill == nil {
		return
	}
	if alpha != nil {
		fill.Alpha = float64(alpha.Val) / 100000.0
	}
	if alphaMod != nil {
		fill.AlphaMod = float64(alphaMod.Val) / 100000.0
	}
	if alphaOff != nil {
		fill.AlphaOff = float64(alphaOff.Val) / 100000.0
	}
	if shade != nil {
		fill.Shade = float64(shade.Val) / 100000.0
		fill.HasShade = true
	}
	if tint != nil {
		fill.Tint = float64(tint.Val) / 100000.0
		fill.HasTint = true
	}
	if red != nil {
		fill.Red = float64(red.Val) / 100000.0
		fill.HasRed = true
	}
	if redMod != nil {
		fill.RedMod = float64(redMod.Val) / 100000.0
		fill.HasRedMod = true
	}
	if redOff != nil {
		fill.RedOff = float64(redOff.Val) / 100000.0
		fill.HasRedOff = true
	}
	if green != nil {
		fill.Green = float64(green.Val) / 100000.0
		fill.HasGreen = true
	}
	if greenMod != nil {
		fill.GreenMod = float64(greenMod.Val) / 100000.0
		fill.HasGreenMod = true
	}
	if greenOff != nil {
		fill.GreenOff = float64(greenOff.Val) / 100000.0
		fill.HasGreenOff = true
	}
	if blue != nil {
		fill.Blue = float64(blue.Val) / 100000.0
		fill.HasBlue = true
	}
	if blueMod != nil {
		fill.BlueMod = float64(blueMod.Val) / 100000.0
		fill.HasBlueMod = true
	}
	if blueOff != nil {
		fill.BlueOff = float64(blueOff.Val) / 100000.0
		fill.HasBlueOff = true
	}
	if hue != nil {
		fill.Hue = float64(hue.Val) / 60000.0
		fill.HasHue = true
	}
	if sat != nil {
		fill.Sat = float64(sat.Val) / 100000.0
		fill.HasSat = true
	}
	if lum != nil {
		fill.Lum = float64(lum.Val) / 100000.0
		fill.HasLum = true
	}
	if lumMod != nil {
		fill.LumMod = float64(lumMod.Val) / 100000.0
	}
	if lumOff != nil {
		fill.LumOff = float64(lumOff.Val) / 100000.0
	}
	if satMod != nil {
		fill.SatMod = float64(satMod.Val) / 100000.0
	}
	if satOff != nil {
		fill.SatOff = float64(satOff.Val) / 100000.0
	}
	if hueMod != nil {
		fill.HueMod = float64(hueMod.Val) / 100000.0
	}
	if hueOff != nil {
		fill.HueOff = float64(hueOff.Val) / 60000.0
	}
	fill.Gray = gray
	fill.Comp = comp
	fill.Inv = inv
	fill.Gamma = gamma
	fill.InvGamma = invGamma
}

// ToDrawingLineStyle converts a Line to a drawing.LineStyle.
func (l *Line) ToDrawingLineStyle() *drawing.LineStyle {
	if l == nil {
		return nil
	}

	result := &drawing.LineStyle{}
	if l.W > 0 {
		result.Width = drawing.EMUToPoints(l.W)
	}
	if l.SolidFill != nil {
		if solidFill := l.SolidFill.ToDrawingSolidFill(); solidFill != nil {
			result.Color = solidFill.Color
			result.ThemeColor = solidFill.ThemeColor
			result.ThemeShade = solidFill.ThemeShade
			result.ThemeTint = solidFill.ThemeTint
			if solidFill.LumMod > 0 {
				result.LumMod = solidFill.LumMod
			}
			result.LumOff = solidFill.LumOff
		}
	}
	if l.GradFill != nil {
		if gf := l.GradFill.ToDrawingGradientFill(); gf != nil && len(gf.Stops) > 0 {
			result.Color = gf.Stops[0].Color
		}
	}
	if l.PattFill != nil {
		if pf := l.PattFill.ToDrawingPatternFill(); pf != nil && pf.Foreground != "" {
			result.Color = pf.Foreground
		}
	}
	if l.PrstDash != nil {
		result.Dash = drawing.DashPattern(l.PrstDash.Val)
	}
	if l.CustDash != nil && len(l.CustDash.Ds) > 0 {
		result.Dash = drawing.DashPattern("dash")
	}
	if l.HeadEnd != nil {
		result.HeadEnd = &drawing.Arrowhead{
			Type:   drawing.ArrowheadType(l.HeadEnd.Type),
			Size:   drawing.ArrowheadSize(l.HeadEnd.W),
			Length: drawing.ArrowheadSize(l.HeadEnd.Len),
		}
	}
	if l.TailEnd != nil {
		result.TailEnd = &drawing.Arrowhead{
			Type:   drawing.ArrowheadType(l.TailEnd.Type),
			Size:   drawing.ArrowheadSize(l.TailEnd.W),
			Length: drawing.ArrowheadSize(l.TailEnd.Len),
		}
	}
	if l.Round != nil {
		result.Join = drawing.LineJoinRound
	}
	if l.Miter != nil {
		result.Join = drawing.LineJoinMiter
		result.MiterLimit = drawing.EMUToPoints(int64(l.Miter.Lim))
	}
	if l.Bevel != nil {
		result.Join = drawing.LineJoinBevel
	}
	switch l.Cap {
	case "rnd", "round":
		result.Cap = drawing.LineCapRound
	case "sq", "square":
		result.Cap = drawing.LineCapSquare
	default:
		result.Cap = drawing.LineCapFlat
	}
	result.Compound = l.Cmpd != "" && l.Cmpd != "sng"
	result.CompoundType = l.Cmpd
	return result
}

// ResolveLineStyle converts a Line to a concrete drawing.LineStyle and
// resolves any theme-aware fill colors against the provided theme color map.
func ResolveLineStyle(l *Line, themeColors map[string]string) *drawing.LineStyle {
	if l == nil || l.NoFill != nil {
		return nil
	}
	result := l.ToDrawingLineStyle()
	result.Color = ""
	result.Alpha = 1.0
	switch {
	case l.SolidFill != nil:
		if fill := ResolveSolidFill(l.SolidFill, themeColors); fill != nil {
			result.Color = fill.Color
			result.Alpha = fill.ResolvedAlpha()
		}
	case l.GradFill != nil:
		if fill := ResolveGradientFill(l.GradFill, themeColors); fill != nil && len(fill.Stops) > 0 {
			result.Color = fill.Stops[0].Color
			result.Alpha = fill.Stops[0].Alpha
		}
	case l.PattFill != nil:
		if fill := ResolvePatternFill(l.PattFill, themeColors); fill != nil {
			result.Color = fill.Foreground
			result.Alpha = fill.ForeAlpha
			if result.Color == "" {
				result.Color = fill.Background
				result.Alpha = fill.BackAlpha
			}
		}
	}
	return result
}

// PresetGeometry represents a:prstGeom or wps:prstGeom — a preset shape geometry.
type PresetGeometry struct {
	XMLName xml.Name        `xml:"prstGeom"`
	Prst    string          `xml:"prst,attr"`
	AvLst   []GeometryGuide `xml:"avLst>gd"`
	AhLst   []AdjustHandle  `xml:"ahLst>ahXY"`
	AhPolar []AdjustHandle  `xml:"ahLst>ahPolar"`
}

// GeometryGuide stores a single adjustment guide (a:gd) inside avLst.
type GeometryGuide struct {
	Name string `xml:"name,attr"`
	Fmla string `xml:"fmla,attr"`
}

// DrawingPresetGeometry converts a PresetGeometry to a drawing.PresetGeometry.
func (pg *PresetGeometry) DrawingPresetGeometry() *drawing.PresetGeometry {
	if pg == nil {
		return nil
	}
	return &drawing.PresetGeometry{
		Name:        pg.Prst,
		Adjustments: orderedPresetAdjustments(pg.AvLst),
		Handles:     append(drawingHandlesFromAdjustHandles(pg.AhLst), drawingHandlesFromAdjustHandles(pg.AhPolar)...),
	}
}

// DrawingGeometry converts a PresetGeometry to a drawing.Geometry.
func (pg *PresetGeometry) DrawingGeometry() *drawing.Geometry {
	if pg == nil {
		return nil
	}
	geom := &drawing.Geometry{
		Type: drawing.GeometryTypePreset,
		Name: pg.Prst,
	}
	geom.Guides = append(geom.Guides, DrawingGuidesFromGeometryGuides(pg.AvLst)...)
	geom.Handles = append(geom.Handles, drawingHandlesFromAdjustHandles(pg.AhLst)...)
	geom.Handles = append(geom.Handles, drawingHandlesFromAdjustHandles(pg.AhPolar)...)
	return geom
}

func orderedPresetAdjustments(src []GeometryGuide) []float64 {
	type adjustment struct {
		order int
		value float64
		index int
	}
	ordered := make([]adjustment, 0, len(src))
	fallback := make([]float64, 0, len(src))
	for i, gd := range src {
		fields := strings.Fields(strings.TrimSpace(gd.Fmla))
		if len(fields) != 2 || fields[0] != "val" {
			continue
		}
		raw, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		value := raw / 100000.0
		if order, ok := presetAdjustmentOrder(gd.Name); ok {
			ordered = append(ordered, adjustment{order: order, value: value, index: i})
			continue
		}
		fallback = append(fallback, value)
	}
	if len(ordered) == 0 {
		return fallback
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].order == ordered[j].order {
			return ordered[i].index < ordered[j].index
		}
		return ordered[i].order < ordered[j].order
	})
	out := make([]float64, 0, len(ordered)+len(fallback))
	for _, adj := range ordered {
		out = append(out, adj.value)
	}
	out = append(out, fallback...)
	return out
}

func presetAdjustmentOrder(name string) (int, bool) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "adj" || name == "adj1" {
		return 0, true
	}
	if !strings.HasPrefix(name, "adj") {
		return 0, false
	}
	n, err := strconv.Atoi(name[3:])
	if err != nil || n < 1 {
		return 0, false
	}
	return n - 1, true
}

// GradientFill represents a:gradFill — a gradient fill definition.
// CustomGeometry describes a DrawingML custom geometry and its guide/path lists.
type CustomGeometry struct {
	AvLst   []GeometryGuide `xml:"avLst>gd"`
	GdLst   []GeometryGuide `xml:"gdLst>gd"`
	AhLst   []AdjustHandle  `xml:"ahLst>ahXY"`
	AhPolar []AdjustHandle  `xml:"ahLst>ahPolar"`
	Paths   []CustomPath    `xml:"pathLst>path"`
}

// CustomPath stores one custom-geometry path in its local coordinate space.
type CustomPath struct {
	W        float64             `xml:"w,attr,omitempty"`
	H        float64             `xml:"h,attr,omitempty"`
	Fill     string              `xml:"fill,attr,omitempty"`
	Commands []CustomPathCommand `xml:"-"`
}

// FormulaPoint stores a custom-path coordinate that may be a formula.
type FormulaPoint struct {
	X string `xml:"x,attr"`
	Y string `xml:"y,attr"`
}

// CustomPathCommand stores one ordered command from a custom geometry path.
type CustomPathCommand struct {
	Type   string
	Points []FormulaPoint
	Args   []string
}

// UnmarshalXML preserves child command order for a custom geometry path.
func (p *CustomPath) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*p = CustomPath{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "w":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				p.W = v
			}
		case "h":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				p.H = v
			}
		case "fill":
			p.Fill = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "moveTo":
				var v struct {
					Pt FormulaPoint `xml:"pt"`
				}
				if err := d.DecodeElement(&v, &se); err != nil {
					return err
				}
				p.Commands = append(p.Commands, CustomPathCommand{Type: "M", Points: []FormulaPoint{v.Pt}})
			case "lnTo":
				var v struct {
					Pt FormulaPoint `xml:"pt"`
				}
				if err := d.DecodeElement(&v, &se); err != nil {
					return err
				}
				p.Commands = append(p.Commands, CustomPathCommand{Type: "L", Points: []FormulaPoint{v.Pt}})
			case "polyLineTo":
				var v struct {
					Pts []FormulaPoint `xml:"pt"`
				}
				if err := d.DecodeElement(&v, &se); err != nil {
					return err
				}
				for _, pt := range v.Pts {
					p.Commands = append(p.Commands, CustomPathCommand{Type: "L", Points: []FormulaPoint{pt}})
				}
			case "cubicBezTo":
				var v struct {
					Pts []FormulaPoint `xml:"pt"`
				}
				if err := d.DecodeElement(&v, &se); err != nil {
					return err
				}
				p.Commands = append(p.Commands, CustomPathCommand{Type: "C", Points: v.Pts})
			case "quadBezTo":
				var v struct {
					Pts []FormulaPoint `xml:"pt"`
				}
				if err := d.DecodeElement(&v, &se); err != nil {
					return err
				}
				p.Commands = append(p.Commands, CustomPathCommand{Type: "Q", Points: v.Pts})
			case "arcTo":
				cmd := CustomPathCommand{Type: "A", Args: make([]string, 4)}
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "wR":
						cmd.Args[0] = attr.Value
					case "hR":
						cmd.Args[1] = attr.Value
					case "stAng":
						cmd.Args[2] = attr.Value
					case "swAng":
						cmd.Args[3] = attr.Value
					}
				}
				p.Commands = append(p.Commands, cmd)
				if err := d.Skip(); err != nil {
					return err
				}
			case "close":
				p.Commands = append(p.Commands, CustomPathCommand{Type: "Z"})
				if err := d.Skip(); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// DrawingGeometry converts a custom geometry into the shared DrawingML form.
// Failed path conversions are silently dropped; use
// DrawingGeometryWithDiagnostics to surface them.
func (g *CustomGeometry) DrawingGeometry() *drawing.Geometry {
	geom, _ := g.DrawingGeometryWithDiagnostics()
	return geom
}

// DrawingGeometryWithDiagnostics converts custom geometry and returns any
// path-conversion failures.
func (g *CustomGeometry) DrawingGeometryWithDiagnostics() (*drawing.Geometry, diag.List) {
	if g == nil {
		return nil, nil
	}
	var diags diag.List
	geom := &drawing.Geometry{Type: drawing.GeometryTypeCustom}
	geom.Guides = append(geom.Guides, DrawingGuidesFromGeometryGuides(g.AvLst)...)
	geom.Guides = append(geom.Guides, DrawingGuidesFromGeometryGuides(g.GdLst)...)
	geom.Handles = append(geom.Handles, drawingHandlesFromAdjustHandles(g.AhLst)...)
	geom.Handles = append(geom.Handles, drawingHandlesFromAdjustHandles(g.AhPolar)...)
	for i, path := range g.Paths {
		converted, err := path.DrawingPath(geom.Guides)
		if err != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "custom-geometry-path-failed",
				Message:  fmt.Sprintf("custom geometry path %d could not be converted", i),
				Cause:    err,
			})
			continue
		}
		geom.Paths = append(geom.Paths, converted)
	}
	return geom, diags
}

// AdjustHandle represents a DrawingML adjustment handle inside ahLst.
type AdjustHandle struct {
	XMLName  xml.Name        `xml:""`
	GDRefX   string          `xml:"gdRefX,attr,omitempty"`
	MinX     string          `xml:"minX,attr,omitempty"`
	MaxX     string          `xml:"maxX,attr,omitempty"`
	GDRefY   string          `xml:"gdRefY,attr,omitempty"`
	MinY     string          `xml:"minY,attr,omitempty"`
	MaxY     string          `xml:"maxY,attr,omitempty"`
	GDRefR   string          `xml:"gdRefR,attr,omitempty"`
	MinR     string          `xml:"minR,attr,omitempty"`
	MaxR     string          `xml:"maxR,attr,omitempty"`
	GDRefAng string          `xml:"gdRefAng,attr,omitempty"`
	MinAng   string          `xml:"minAng,attr,omitempty"`
	MaxAng   string          `xml:"maxAng,attr,omitempty"`
	Pos      AdjustHandlePos `xml:"pos"`
}

// UnmarshalXML preserves whether this handle came from ahXY or ahPolar.
func (h *AdjustHandle) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	type alias AdjustHandle
	aux := alias{}
	if err := dec.DecodeElement(&aux, &start); err != nil {
		return err
	}
	*h = AdjustHandle(aux)
	h.XMLName = start.Name
	return nil
}

// AdjustHandlePos stores the authored handle position reference.
type AdjustHandlePos struct {
	XMLName xml.Name `xml:"pos"`
	X       string   `xml:"x,attr,omitempty"`
	Y       string   `xml:"y,attr,omitempty"`
}

// DrawingGuidesFromGeometryGuides converts DrawingML guides to the shared
// drawing-guide representation.
func DrawingGuidesFromGeometryGuides(src []GeometryGuide) []drawing.Guide {
	out := make([]drawing.Guide, 0, len(src))
	for _, gd := range src {
		guide := drawing.Guide{Name: gd.Name}
		fields := strings.Fields(strings.TrimSpace(gd.Fmla))
		if len(fields) == 2 && fields[0] == "val" {
			if raw, err := strconv.ParseFloat(fields[1], 64); err == nil {
				guide.Value = raw
			} else {
				guide.Formula = gd.Fmla
			}
		} else {
			guide.Formula = gd.Fmla
		}
		out = append(out, guide)
	}
	return out
}

func drawingHandlesFromAdjustHandles(src []AdjustHandle) []drawing.AdjustHandle {
	out := make([]drawing.AdjustHandle, 0, len(src))
	for _, handle := range src {
		kind := strings.TrimSpace(handle.XMLName.Local)
		out = append(out, drawing.AdjustHandle{
			Kind:     kind,
			PosX:     handle.Pos.X,
			PosY:     handle.Pos.Y,
			GDRefX:   handle.GDRefX,
			MinX:     handle.MinX,
			MaxX:     handle.MaxX,
			GDRefY:   handle.GDRefY,
			MinY:     handle.MinY,
			MaxY:     handle.MaxY,
			GDRefR:   handle.GDRefR,
			MinR:     handle.MinR,
			MaxR:     handle.MaxR,
			GDRefAng: handle.GDRefAng,
			MinAng:   handle.MinAng,
			MaxAng:   handle.MaxAng,
		})
	}
	return out
}

// DrawingPath converts a custom path into a normalized drawing path.
func (p *CustomPath) DrawingPath(guides []drawing.Guide) (drawing.Path, error) {
	width := p.W
	height := p.H
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	guideValues, err := (&drawing.Geometry{Guides: guides}).EvaluateGuides(width, height)
	if err != nil {
		guideValues = map[string]float64{}
	}
	ctx := drawing.GuideContext{Width: width, Height: height, Guides: guideValues}
	path := drawing.Path{FillRule: "nonzero"}
	current := drawing.Point{}
	for _, cmd := range p.Commands {
		switch cmd.Type {
		case "M", "L", "C", "Q":
			pts := make([]drawing.Point, 0, len(cmd.Points))
			for _, pt := range cmd.Points {
				abs, err := EvaluateCustomPoint(pt, ctx)
				if err != nil {
					return drawing.Path{}, err
				}
				current = abs
				pts = append(pts, drawing.Point{X: abs.X / width, Y: abs.Y / height})
			}
			path.Commands = append(path.Commands, drawing.PathCommand{Type: cmd.Type, Points: pts})
		case "A":
			arc, next, err := CustomArcCommand(cmd, current, ctx, width, height)
			if err != nil {
				return drawing.Path{}, err
			}
			current = next
			path.Commands = append(path.Commands, arc)
		case "Z":
			path.Commands = append(path.Commands, drawing.PathCommand{Type: "Z"})
		}
	}
	return path, nil
}

// CustomArcCommand converts an OOXML arc path command into a normalized arc.
func CustomArcCommand(cmd CustomPathCommand, current drawing.Point, ctx drawing.GuideContext, width, height float64) (drawing.PathCommand, drawing.Point, error) {
	if len(cmd.Args) < 4 {
		return drawing.PathCommand{}, current, nil
	}
	wR, err := EvaluateCustomValue(cmd.Args[0], ctx)
	if err != nil {
		return drawing.PathCommand{}, current, err
	}
	hR, err := EvaluateCustomValue(cmd.Args[1], ctx)
	if err != nil {
		return drawing.PathCommand{}, current, err
	}
	stAng, err := EvaluateCustomValue(cmd.Args[2], ctx)
	if err != nil {
		return drawing.PathCommand{}, current, err
	}
	swAng, err := EvaluateCustomValue(cmd.Args[3], ctx)
	if err != nil {
		return drawing.PathCommand{}, current, err
	}
	startRad := stAng / 60000.0 * math.Pi / 180.0
	endRad := (stAng + swAng) / 60000.0 * math.Pi / 180.0
	center := drawing.Point{X: current.X - wR*math.Cos(startRad), Y: current.Y - hR*math.Sin(startRad)}
	end := drawing.Point{X: center.X + wR*math.Cos(endRad), Y: center.Y + hR*math.Sin(endRad)}
	largeArc := 0.0
	if math.Abs(swAng) > 180*60000 {
		largeArc = 1
	}
	sweep := 0.0
	if swAng >= 0 {
		sweep = 1
	}
	return drawing.PathCommand{
		Type: "A",
		Points: []drawing.Point{{
			X: end.X / width,
			Y: end.Y / height,
		}},
		Args: []float64{wR / width, hR / height, 0, largeArc, sweep},
	}, end, nil
}

// EvaluateCustomPoint resolves a formula point inside a custom path.
func EvaluateCustomPoint(pt FormulaPoint, ctx drawing.GuideContext) (drawing.Point, error) {
	x, err := EvaluateCustomValue(pt.X, ctx)
	if err != nil {
		return drawing.Point{}, err
	}
	y, err := EvaluateCustomValue(pt.Y, ctx)
	if err != nil {
		return drawing.Point{}, err
	}
	return drawing.Point{X: x, Y: y}, nil
}

// EvaluateCustomValue resolves a formula or literal value used in custom
// geometry coordinates and arc arguments.
func EvaluateCustomValue(expr string, ctx drawing.GuideContext) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, nil
	}
	if value, err := strconv.ParseFloat(expr, 64); err == nil {
		return value, nil
	}
	return drawing.EvaluateFormula(expr, ctx)
}

type GradientFill struct {
	XMLName         xml.Name        `xml:"gradFill"`
	Flip            string          `xml:"flip,attr,omitempty"`
	RotateWithShape *string         `xml:"rotWithShape,attr,omitempty"`
	StopList        []GradientStop  `xml:"gsLst>gs"`
	Linear          *LinearGradient `xml:"lin"`
	Path            *PathGradient   `xml:"path"`
}

// GradientStop represents a:gs — a single gradient stop.
type GradientStop struct {
	XMLName   xml.Name   `xml:"gs"`
	Pos       int        `xml:"pos,attr"`
	SRGBClr   *SRGBClr   `xml:"srgbClr"`
	SchemeClr *SchemeClr `xml:"schemeClr"`
	SysClr    *SysClr    `xml:"sysClr"`
	PrstClr   *PrstClr   `xml:"prstClr"`
	ScrGBClr  *ScrGBClr  `xml:"scrgbClr"`
	HSLClr    *HSLClr    `xml:"hslClr"`
}

// LinearGradient represents a:lin — linear gradient parameters.
type LinearGradient struct {
	XMLName xml.Name `xml:"lin"`
	Angle   int      `xml:"ang,attr"`
	Scaled  *string  `xml:"scaled,attr,omitempty"`
}

// PathGradient represents a:path — path (radial) gradient parameters.
type PathGradient struct {
	XMLName xml.Name `xml:"path"`
	Path    string   `xml:"path,attr,omitempty"`
}

// ToDrawingGradientFill converts a GradientFill to a drawing.GradientFill.
func (gf *GradientFill) ToDrawingGradientFill() *drawing.GradientFill {
	if gf == nil {
		return nil
	}
	result := &drawing.GradientFill{
		GradientType: drawing.GradientTypeLinear,
		Flip:         gf.Flip,
	}
	if gf.RotateWithShape != nil {
		result.RotateWithShape = *gf.RotateWithShape == "1" || *gf.RotateWithShape == "true"
	}
	if gf.Linear != nil {
		result.Angle = float64(gf.Linear.Angle) / 60000.0
	}
	if gf.Path != nil {
		switch gf.Path.Path {
		case "circle", "rect", "shape":
			result.GradientType = drawing.GradientTypeRadial
		default:
			result.GradientType = drawing.GradientTypePath
		}
	}
	for _, stop := range gf.StopList {
		ds := drawing.GradientStop{Position: float64(stop.Pos) / 100000.0, Alpha: 1.0}
		if fill := stop.toDrawingSolidFill(); fill != nil {
			ds.Color = fill.Color
			if ds.Color == "" {
				ds.Color = fill.ThemeColor
			}
			ds.Alpha = fill.ResolvedAlpha()
		}
		result.Stops = append(result.Stops, ds)
	}
	return result
}

// ResolveGradientFill converts a GradientFill to a concrete
// drawing.GradientFill and resolves any theme-color references against the
// provided theme color map.
func ResolveGradientFill(gf *GradientFill, themeColors map[string]string) *drawing.GradientFill {
	if gf == nil {
		return nil
	}
	result := gf.ToDrawingGradientFill()
	for i, stop := range gf.StopList {
		resolved := resolveDrawingSolidFill(stop.toDrawingSolidFill(), themeColors)
		if resolved == nil {
			result.Stops[i].Color = ""
			result.Stops[i].Alpha = 1.0
			continue
		}
		result.Stops[i].Color = resolved.Color
		result.Stops[i].Alpha = resolved.ResolvedAlpha()
	}
	return result
}

func (gs *GradientStop) toDrawingSolidFill() *drawing.SolidFill {
	if gs == nil {
		return nil
	}
	switch {
	case gs.SRGBClr != nil:
		return gs.SRGBClr.toDrawingSolidFill()
	case gs.SchemeClr != nil:
		return gs.SchemeClr.toDrawingSolidFill()
	case gs.SysClr != nil:
		return gs.SysClr.toDrawingSolidFill()
	case gs.PrstClr != nil:
		return gs.PrstClr.toDrawingSolidFill()
	case gs.ScrGBClr != nil:
		return gs.ScrGBClr.toDrawingSolidFill()
	case gs.HSLClr != nil:
		return gs.HSLClr.toDrawingSolidFill()
	}
	return nil
}

// PatternFill represents a:pattFill — a pattern fill definition.
type PatternFill struct {
	XMLName xml.Name     `xml:"pattFill"`
	Prst    string       `xml:"prst,attr,omitempty"`
	FgClr   *ColorChoice `xml:"fgClr"`
	BgClr   *ColorChoice `xml:"bgClr"`
}

// ColorChoice represents a:fgClr or a:bgClr inside a pattern fill.
type ColorChoice struct {
	SRGBClr   *SRGBClr   `xml:"srgbClr"`
	SchemeClr *SchemeClr `xml:"schemeClr"`
	SysClr    *SysClr    `xml:"sysClr"`
	PrstClr   *PrstClr   `xml:"prstClr"`
	ScrGBClr  *ScrGBClr  `xml:"scrgbClr"`
	HSLClr    *HSLClr    `xml:"hslClr"`
}

// BlipFill stores a picture/blip fill specification.
type BlipFill struct {
	XMLName      xml.Name `xml:"blipFill"`
	Dpi          int      `xml:"dpi,attr,omitempty"`
	RotWithShape *string  `xml:"rotWithShape,attr,omitempty"`
	Blip         *BlipRef `xml:"blip"`
	SrcRect      *SrcRect `xml:"srcRect"`
	Tile         *Tile    `xml:"tile"`
	Stretch      *Stretch `xml:"stretch"`
}

// BlipRef stores the relationship reference inside a:blip.
type BlipRef struct {
	XMLName xml.Name `xml:"blip"`
	Embed   string   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
}

// SrcRect stores picture crop percentages in thousandths of a percent.
type SrcRect struct {
	XMLName xml.Name `xml:"srcRect"`
	L       int      `xml:"l,attr,omitempty"`
	T       int      `xml:"t,attr,omitempty"`
	R       int      `xml:"r,attr,omitempty"`
	B       int      `xml:"b,attr,omitempty"`
}

// Tile stores tiling parameters for a blip fill.
type Tile struct {
	XMLName xml.Name `xml:"tile"`
	Tx      int      `xml:"tx,attr,omitempty"`
	Ty      int      `xml:"ty,attr,omitempty"`
	Sx      int      `xml:"sx,attr,omitempty"`
	Sy      int      `xml:"sy,attr,omitempty"`
	Flip    string   `xml:"flip,attr,omitempty"`
	Algn    string   `xml:"algn,attr,omitempty"`
}

// Stretch stores stretch parameters for a blip fill.
type Stretch struct {
	XMLName  xml.Name  `xml:"stretch"`
	FillRect *FillRect `xml:"fillRect"`
}

// FillRect stores the fill-to rectangle for a stretch.
type FillRect struct {
	XMLName xml.Name `xml:"fillRect"`
	L       int      `xml:"l,attr,omitempty"`
	T       int      `xml:"t,attr,omitempty"`
	R       int      `xml:"r,attr,omitempty"`
	B       int      `xml:"b,attr,omitempty"`
}

// ResolveBlipFill converts a BlipFill to a concrete drawing.PictureFill.
func ResolveBlipFill(bf *BlipFill) *drawing.PictureFill {
	if bf == nil || bf.Blip == nil || strings.TrimSpace(bf.Blip.Embed) == "" {
		return nil
	}
	fill := &drawing.PictureFill{
		EmbedID: bf.Blip.Embed,
		Alpha:   1.0,
		Tile:    bf.Tile != nil,
		Stretch: bf.Stretch != nil || (bf.Tile == nil && bf.Stretch == nil),
	}
	if bf.Tile != nil {
		fill.TileOffsetX = int64(bf.Tile.Tx)
		fill.TileOffsetY = int64(bf.Tile.Ty)
		fill.TileScaleX = bf.Tile.Sx
		fill.TileScaleY = bf.Tile.Sy
		fill.TileFlip = bf.Tile.Flip
		fill.TileAlign = bf.Tile.Algn
	}
	if bf.SrcRect != nil {
		left := clampBlipCropPct(bf.SrcRect.L)
		top := clampBlipCropPct(bf.SrcRect.T)
		right := clampBlipCropPct(bf.SrcRect.R)
		bottom := clampBlipCropPct(bf.SrcRect.B)
		width := 1 - left - right
		height := 1 - top - bottom
		if width < 0 {
			width = 0
		}
		if height < 0 {
			height = 0
		}
		fill.Crop = &drawing.Rect{
			X:      left,
			Y:      top,
			Width:  width,
			Height: height,
		}
	}
	if bf.Stretch != nil && bf.Stretch.FillRect != nil {
		left := clampBlipCropPct(bf.Stretch.FillRect.L)
		top := clampBlipCropPct(bf.Stretch.FillRect.T)
		right := clampBlipCropPct(bf.Stretch.FillRect.R)
		bottom := clampBlipCropPct(bf.Stretch.FillRect.B)
		width := 1 - left - right
		height := 1 - top - bottom
		if width < 0 {
			width = 0
		}
		if height < 0 {
			height = 0
		}
		fill.FillRect = &drawing.Rect{X: left, Y: top, Width: width, Height: height}
	}
	return fill
}

// normalizePatternType maps OOXML a:pattFill@prst values to drawing.PatternType names.
func normalizePatternType(prst string) string {
	switch prst {
	case "pct5":
		return "percent5"
	case "pct10":
		return "percent10"
	case "pct20":
		return "percent20"
	case "pct25":
		return "percent25"
	case "pct30":
		return "percent30"
	case "pct40":
		return "percent40"
	case "pct50":
		return "percent50"
	case "pct60":
		return "percent60"
	case "pct70":
		return "percent70"
	case "pct75":
		return "percent75"
	case "pct80":
		return "percent80"
	case "pct90":
		return "percent90"
	case "dkHorz":
		return "darkHorizontal"
	case "dkVert":
		return "darkVertical"
	case "dkDnDiag":
		return "darkDownDiagonal"
	case "dkUpDiag":
		return "darkUpDiagonal"
	case "smGrid":
		return "smallGrid"
	case "lgGrid":
		return "largeGrid"
	case "dotGrid":
		return "dottedGrid"
	case "dashHorz":
		return "dashedHorizontal"
	case "dashVert":
		return "dashedVertical"
	case "narHorz":
		return "lightHorizontal"
	case "narVert":
		return "lightVertical"
	case "dashDnDiag":
		return "dashedDownDiagonal"
	case "dashUpDiag":
		return "dashedUpDiagonal"
	case "lgConfetti":
		return "largeConfetti"
	case "smConfetti":
		return "smallConfetti"
	case "horzBrick":
		return "horizontalBrick"
	case "diagBrick":
		return "diagonalBrick"
	case "dotDmnd":
		return "dottedDiamond"
	case "smCheck":
		return "smallCheckerBoard"
	case "lgCheck":
		return "largeCheckerBoard"
	case "solidDmnd":
		return "solidDiamond"
	case "openDmnd":
		return "openDiamond"
	case "diagCross":
		return "diagonalCross"
	}
	return prst
}

// ToDrawingPatternFill converts a PatternFill to a drawing.PatternFill.
func (pf *PatternFill) ToDrawingPatternFill() *drawing.PatternFill {
	if pf == nil {
		return nil
	}
	result := &drawing.PatternFill{
		PatternType: drawing.PatternType(normalizePatternType(pf.Prst)),
	}
	if pf.FgClr != nil {
		if fill := pf.FgClr.toDrawingSolidFill(); fill != nil {
			result.Foreground = fill.Color
			if result.Foreground == "" {
				result.Foreground = fill.ThemeColor
			}
		}
	}
	if pf.BgClr != nil {
		if fill := pf.BgClr.toDrawingSolidFill(); fill != nil {
			result.Background = fill.Color
			if result.Background == "" {
				result.Background = fill.ThemeColor
			}
		}
	}
	return result
}

// ResolvePatternFill converts a PatternFill to a concrete drawing.PatternFill
// and resolves any theme-color references against the provided theme color map.
func ResolvePatternFill(pf *PatternFill, themeColors map[string]string) *drawing.PatternFill {
	if pf == nil {
		return nil
	}
	result := pf.ToDrawingPatternFill()
	if fill := resolveColorChoiceFill(pf.FgClr, themeColors); fill != nil {
		result.Foreground = fill.Color
		result.ForeAlpha = fill.ResolvedAlpha()
	} else {
		result.Foreground = ""
	}
	if fill := resolveColorChoiceFill(pf.BgClr, themeColors); fill != nil {
		result.Background = fill.Color
		result.BackAlpha = fill.ResolvedAlpha()
	} else {
		result.Background = ""
	}
	return result
}

func (cc *ColorChoice) toDrawingSolidFill() *drawing.SolidFill {
	if cc == nil {
		return nil
	}
	switch {
	case cc.SRGBClr != nil:
		return cc.SRGBClr.toDrawingSolidFill()
	case cc.SchemeClr != nil:
		return cc.SchemeClr.toDrawingSolidFill()
	case cc.SysClr != nil:
		return cc.SysClr.toDrawingSolidFill()
	case cc.PrstClr != nil:
		return cc.PrstClr.toDrawingSolidFill()
	case cc.ScrGBClr != nil:
		return cc.ScrGBClr.toDrawingSolidFill()
	case cc.HSLClr != nil:
		return cc.HSLClr.toDrawingSolidFill()
	}
	return nil
}

func resolveColorChoiceFill(choice *ColorChoice, themeColors map[string]string) *drawing.SolidFill {
	if choice == nil {
		return nil
	}
	return resolveDrawingSolidFill(choice.toDrawingSolidFill(), themeColors)
}

func resolveDrawingSolidFill(fill *drawing.SolidFill, themeColors map[string]string) *drawing.SolidFill {
	if fill == nil {
		return nil
	}
	resolved := *fill
	base := strings.TrimSpace(resolved.Color)
	if base == "" && resolved.ThemeColor != "" && themeColors != nil {
		base = strings.TrimSpace(themeColors[resolved.ThemeColor])
	}
	if base == "" {
		return &resolved
	}
	resolved.Color = resolved.ApplyColorTransforms(base)
	return &resolved
}

func clampBlipCropPct(value int) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100000 {
		return 1
	}
	return float64(value) / 100000.0
}

func formatHexRGBFromPercent(r, g, b int) string {
	return fmt.Sprintf("%02X%02X%02X", percentToByte(r), percentToByte(g), percentToByte(b))
}

func percentToByte(value int) uint8 {
	clamped := clampPercent100000(value)
	return uint8(math.Round((clamped / 100000.0) * 255.0))
}

func clampPercent100000(value int) float64 {
	if value < 0 {
		return 0
	}
	if value > 100000 {
		return 100000
	}
	return float64(value)
}

func formatHexRGBFromHSL(hue, sat, lum int) string {
	h := math.Mod(float64(hue)/60000.0, 360.0)
	if h < 0 {
		h += 360
	}
	s := clampPercent100000(sat) / 100000.0
	l := clampPercent100000(lum) / 100000.0
	r, g, b := hslToRGB(h, s, l)
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

func hslToRGB(h, s, l float64) (r, g, b uint8) {
	if s == 0 {
		gray := uint8(math.Round(l * 255.0))
		return gray, gray, gray
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hk := h / 360.0
	rf := hueToRGB(p, q, hk+1.0/3.0)
	gf := hueToRGB(p, q, hk)
	bf := hueToRGB(p, q, hk-1.0/3.0)
	return uint8(math.Round(rf * 255.0)), uint8(math.Round(gf * 255.0)), uint8(math.Round(bf * 255.0))
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

func presetColorHex(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "black":
		return "000000"
	case "white":
		return "FFFFFF"
	case "red":
		return "FF0000"
	case "green":
		return "008000"
	case "blue":
		return "0000FF"
	case "yellow":
		return "FFFF00"
	case "cyan":
		return "00FFFF"
	case "magenta":
		return "FF00FF"
	case "gray", "grey":
		return "808080"
	case "ltgray", "ltgrey", "lightgray", "lightgrey":
		return "D3D3D3"
	case "dkgray", "dkgrey", "darkgray", "darkgrey":
		return "A9A9A9"
	case "ltblue", "lightblue":
		return "ADD8E6"
	case "dkblue", "darkblue", "navy":
		return "00008B"
	case "ltgreen", "lightgreen":
		return "90EE90"
	case "dkgreen", "darkgreen":
		return "006400"
	case "ltcyan", "lightcyan":
		return "E0FFFF"
	case "dkcyan", "darkcyan", "teal":
		return "008B8B"
	case "ltyellow", "lightyellow":
		return "FFFFE0"
	case "dkyellow", "darkyellow", "olive":
		return "808000"
	case "orange":
		return "FFA500"
	case "brown":
		return "A52A2A"
	case "purple":
		return "800080"
	case "pink":
		return "FFC0CB"
	default:
		return ""
	}
}

func systemColorHex(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "window", "windowtext", "menutext", "btntext", "captiontext":
		return "000000"
	case "windowframe", "activeborder", "inactiveborder":
		return "808080"
	case "background", "btnface", "menu", "scrollbar", "3dlight":
		return "FFFFFF"
	case "highlight":
		return "3399FF"
	case "highlighttext":
		return "FFFFFF"
	case "graytext", "disabledtext", "inactivecaptiontext":
		return "808080"
	case "hotlight":
		return "0000FF"
	case "infobk":
		return "FFFFE1"
	case "infotext":
		return "000000"
	default:
		return ""
	}
}
