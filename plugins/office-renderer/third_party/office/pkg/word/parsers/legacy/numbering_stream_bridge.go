package legacy

import (
	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

// legacyListTablesFromCanonical adapts the canonical Word97 numbering catalog
// into the temporary legacy projection shape. It intentionally performs no
// FIB/table parsing: PlfLst/PlfLfo ownership stays in doc/word97.
func legacyListTablesFromCanonical(catalog *word97.NumberingCatalog) *legacyListTables {
	if catalog == nil || len(catalog.Instances) == 0 || len(catalog.Abstract) == 0 {
		return nil
	}

	tables := &legacyListTables{
		formatByIlfoLevel:                 map[string]string{},
		startByIlfoLevel:                  map[string]int{},
		restartByIlfoLevel:                map[string]bool{},
		restartOnLevelByIlfoLevel:         map[string]int{},
		indentLeftByIlfoLevel:             map[string]int{},
		indentFirstLineByIlfoLevel:        map[string]int{},
		templateByIlfoLevel:               map[string]string{},
		lsidByIlfo:                        map[int]uint32{},
		runFormatByIlfoLevel:              map[string]*wordir.RunFormat{},
		overrideStartByIlfoLevel:          map[string]int{},
		overrideFormatByIlfoLevel:         map[string]string{},
		overrideRestartByIlfoLevel:        map[string]bool{},
		overrideRestartOnLevelByIlfoLevel: map[string]int{},
	}

	for ilfo, instance := range catalog.Instances {
		if ilfo <= 0 {
			continue
		}
		tables.lsidByIlfo[ilfo] = uint32(instance.AbstractNumID)
		levels := catalog.Abstract[instance.AbstractNumID]
		for _, level := range levels {
			if level.Level < 0 {
				continue
			}
			key := legacyListKey(ilfo, level.Level)
			format := level.Format
			start := level.Start
			if override, ok := instance.Overrides[level.Level]; ok {
				if override.Start != nil {
					start = *override.Start
					tables.overrideStartByIlfoLevel[key] = *override.Start
				}
				if override.Format != "" {
					format = override.Format
					tables.overrideFormatByIlfoLevel[key] = override.Format
				}
				if override.Restart != nil {
					tables.overrideRestartByIlfoLevel[key] = *override.Restart
				}
				if override.RestartOnLevel != nil {
					tables.overrideRestartOnLevelByIlfoLevel[key] = *override.RestartOnLevel
				}
			}
			if format != "" {
				tables.formatByIlfoLevel[key] = format
			}
			if start > 0 {
				tables.startByIlfoLevel[key] = start
			}
			if level.RestartOnLevel >= 0 {
				tables.restartOnLevelByIlfoLevel[key] = level.RestartOnLevel
			} else if level.Level > 0 && !level.NoRestart {
				// Word's default multilevel behavior restarts a subordinate level
				// after movement through a higher level when no explicit limit is
				// encoded.
				tables.restartByIlfoLevel[key] = true
			}
			if level.IndentLeft != 0 {
				tables.indentLeftByIlfoLevel[key] = level.IndentLeft
			}
			firstLine := level.IndentFirstLine
			if firstLine == 0 && level.IndentHanging > 0 {
				firstLine = -level.IndentHanging
			}
			if firstLine != 0 {
				tables.indentFirstLineByIlfoLevel[key] = firstLine
			}
			if level.Text != "" {
				tables.templateByIlfoLevel[key] = level.Text
			}
			if level.RunFormat != nil {
				rf := cloneCanonicalNumberingRunFormat(level.RunFormat)
				tables.runFormatByIlfoLevel[key] = &rf
			}
		}
	}

	if len(tables.formatByIlfoLevel) == 0 {
		return nil
	}
	return tables
}

func cloneCanonicalNumberingRunFormat(src *wordir.RunFormat) wordir.RunFormat {
	if src == nil {
		return wordir.RunFormat{}
	}
	out := *src
	cloneBool := func(p *bool) *bool {
		if p == nil { return nil }
		v := *p
		return &v
	}
	cloneInt := func(p *int) *int {
		if p == nil { return nil }
		v := *p
		return &v
	}
	out.Bold = cloneBool(src.Bold)
	out.Italic = cloneBool(src.Italic)
	out.Caps = cloneBool(src.Caps)
	out.SmallCaps = cloneBool(src.SmallCaps)
	out.Strike = cloneBool(src.Strike)
	out.FontSize = cloneInt(src.FontSize)
	out.Vanish = cloneBool(src.Vanish)
	out.CharacterSpacing = cloneInt(src.CharacterSpacing)
	out.RTL = cloneBool(src.RTL)
	out.Outline = cloneBool(src.Outline)
	out.Shadow = cloneBool(src.Shadow)
	out.Emboss = cloneBool(src.Emboss)
	out.Imprint = cloneBool(src.Imprint)
	return out
}
