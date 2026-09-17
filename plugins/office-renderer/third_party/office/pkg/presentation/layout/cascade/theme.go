// Package cascade owns presentation visual inheritance that is independent of
// the OOXML projection machinery.  It resolves the effective theme and color
// maps for a slide across the presentation -> master -> layout -> slide chain.
package cascade

import (
	"strings"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// ThemeContext is the effective visual theme state for one slide.
type ThemeContext struct {
	RawTheme     *parts.Theme
	Theme        *ir.Theme
	MasterColors map[string]string
	LayoutColors map[string]string
	SlideColors  map[string]string
}

// ResolveThemeContext resolves theme overrides and color-map overrides in the
// order defined by PresentationML.  Downstream shape/text resolution consumes
// this context instead of reconstructing the cascade itself.
func ResolveThemeContext(base *parts.Theme, master *parts.SlideMaster, layout *parts.SlideLayout, slide *parts.Slide) ThemeContext {
	layoutTheme := ApplyThemeOverride(base, themeOverrideForLayout(layout))
	slideTheme := ApplyThemeOverride(layoutTheme, themeOverrideForSlide(slide))
	resolved := ResolveTheme(slideTheme)

	baseColors := map[string]string{}
	if resolved != nil {
		baseColors = BuildThemeColorMap(resolved.ColorScheme, nil)
	}
	masterColors := baseColors
	if master != nil {
		masterColors = BuildThemeColorMap(baseColors, master.ClrMap)
	}
	layoutColors := ApplyColorMapOverride(baseColors, masterColors, colorMapOverrideForLayout(layout))
	slideColors := ApplyColorMapOverride(baseColors, layoutColors, colorMapOverrideForSlide(slide))

	return ThemeContext{
		RawTheme:     slideTheme,
		Theme:        resolved,
		MasterColors: masterColors,
		LayoutColors: layoutColors,
		SlideColors:  slideColors,
	}
}

func ResolveTheme(t *parts.Theme) *ir.Theme {
	if t == nil {
		return nil
	}
	out := &ir.Theme{
		Name: t.Name, ColorScheme: make(map[string]string),
		MajorFont:   t.Elements.FontScheme.MajorFont.Latin.Typeface,
		MinorFont:   t.Elements.FontScheme.MinorFont.Latin.Typeface,
		MajorEAFont: t.Elements.FontScheme.MajorFont.EA.Typeface,
		MinorEAFont: t.Elements.FontScheme.MinorFont.EA.Typeface,
		MajorCSFont: t.Elements.FontScheme.MajorFont.CS.Typeface,
		MinorCSFont: t.Elements.FontScheme.MinorFont.CS.Typeface,
	}
	cs := t.Elements.ClrScheme
	resolveThemeColor(out.ColorScheme, "dk1", cs.DK1)
	resolveThemeColor(out.ColorScheme, "lt1", cs.LT1)
	resolveThemeColor(out.ColorScheme, "dk2", cs.DK2)
	resolveThemeColor(out.ColorScheme, "lt2", cs.LT2)
	resolveThemeColor(out.ColorScheme, "accent1", cs.Accent1)
	resolveThemeColor(out.ColorScheme, "accent2", cs.Accent2)
	resolveThemeColor(out.ColorScheme, "accent3", cs.Accent3)
	resolveThemeColor(out.ColorScheme, "accent4", cs.Accent4)
	resolveThemeColor(out.ColorScheme, "accent5", cs.Accent5)
	resolveThemeColor(out.ColorScheme, "accent6", cs.Accent6)
	resolveThemeColor(out.ColorScheme, "hlink", cs.HLink)
	resolveThemeColor(out.ColorScheme, "folHlink", cs.FolHLink)
	return out
}

func ApplyThemeOverride(base *parts.Theme, override *parts.ThemeOverride) *parts.Theme {
	if base == nil || override == nil {
		return base
	}
	merged := *base
	merged.Elements = base.Elements
	if override.ClrScheme != nil {
		merged.Elements.ClrScheme = *override.ClrScheme
	}
	if override.FontScheme != nil {
		merged.Elements.FontScheme = *override.FontScheme
	}
	if override.FmtScheme != nil {
		merged.Elements.FmtScheme = *override.FmtScheme
	}
	return &merged
}

func BuildThemeColorMap(base map[string]string, colorMap *parts.ColorMap) map[string]string {
	out := make(map[string]string, len(base)+16)
	for k, v := range base {
		SetThemeColor(out, k, v)
	}
	aliases := presentationColorMapAliases()
	if colorMap != nil {
		for k, v := range colorMapAliases(colorMap) {
			aliases[k] = v
		}
	}
	for alias, target := range aliases {
		SetThemeColor(out, alias, ThemeColor(base, target))
	}
	return out
}

func ApplyColorMapOverride(base, inherited map[string]string, override *parts.ColorMapOverride) map[string]string {
	if override == nil || override.MasterClrMapping != nil || override.OverrideClrMapping == nil {
		return inherited
	}
	return BuildThemeColorMap(base, &override.OverrideClrMapping.ColorMap)
}

func ThemeColor(colors map[string]string, key string) string {
	if colors == nil {
		return ""
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if v := strings.TrimSpace(colors[key]); v != "" {
		return v
	}
	return strings.TrimSpace(colors[strings.ToLower(key)])
}

func SetThemeColor(colors map[string]string, key, value string) {
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	colors[key] = value
	colors[strings.ToLower(key)] = value
}

func resolveThemeColor(m map[string]string, slot string, tc *parts.ThemeColor) {
	if tc == nil {
		return
	}
	if tc.SrgbClr != nil {
		m[slot] = tc.SrgbClr.Val
		return
	}
	if tc.SysClr != nil && tc.SysClr.LastClr != "" {
		m[slot] = tc.SysClr.LastClr
	}
}
func themeOverrideForLayout(v *parts.SlideLayout) *parts.ThemeOverride {
	if v == nil {
		return nil
	}
	return v.ThemeOverride
}
func themeOverrideForSlide(v *parts.Slide) *parts.ThemeOverride {
	if v == nil {
		return nil
	}
	return v.ThemeOverride
}
func colorMapOverrideForLayout(v *parts.SlideLayout) *parts.ColorMapOverride {
	if v == nil {
		return nil
	}
	return v.ClrMapOvr
}
func colorMapOverrideForSlide(v *parts.Slide) *parts.ColorMapOverride {
	if v == nil {
		return nil
	}
	return v.ClrMapOvr
}
func presentationColorMapAliases() map[string]string {
	return map[string]string{"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2", "accent1": "accent1", "accent2": "accent2", "accent3": "accent3", "accent4": "accent4", "accent5": "accent5", "accent6": "accent6", "hlink": "hlink", "followedHlink": "folHlink", "folHlink": "folHlink"}
}
func colorMapAliases(c *parts.ColorMap) map[string]string {
	if c == nil {
		return nil
	}
	return map[string]string{"bg1": strings.TrimSpace(c.Bg1), "tx1": strings.TrimSpace(c.Tx1), "bg2": strings.TrimSpace(c.Bg2), "tx2": strings.TrimSpace(c.Tx2), "accent1": strings.TrimSpace(c.Accent1), "accent2": strings.TrimSpace(c.Accent2), "accent3": strings.TrimSpace(c.Accent3), "accent4": strings.TrimSpace(c.Accent4), "accent5": strings.TrimSpace(c.Accent5), "accent6": strings.TrimSpace(c.Accent6), "hlink": strings.TrimSpace(c.Hlink), "followedHlink": strings.TrimSpace(c.FollowedHlink), "folHlink": strings.TrimSpace(c.FollowedHlink)}
}
