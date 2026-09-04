package legacy

import (
	"encoding/binary"
	"sort"
	"strconv"
	"strings"

	wordir "office/pkg/word/ir"
)

type legacyListTables struct {
	formatByIlfoLevel                 map[string]string
	startByIlfoLevel                  map[string]int
	restartByIlfoLevel                map[string]bool
	restartOnLevelByIlfoLevel         map[string]int
	indentLeftByIlfoLevel             map[string]int               // left indent from LVLF PAPx grpprl (twips)
	indentFirstLineByIlfoLevel        map[string]int               // first-line indent from LVLF PAPx grpprl (twips; negative = hanging)
	templateByIlfoLevel               map[string]string            // LVLF xst template, e.g. "%1.%2." (level placeholders are 1-indexed)
	lsidByIlfo                        map[int]uint32               // ilfo → lsid (list ID) mapping for AbstractNumID convergence
	runFormatByIlfoLevel              map[string]*wordir.RunFormat // per-level marker character formatting from LVLF CHPx; nil = no CHPx grpprl present
	overrideStartByIlfoLevel          map[string]int
	overrideFormatByIlfoLevel         map[string]string
	overrideRestartByIlfoLevel        map[string]bool
	overrideRestartOnLevelByIlfoLevel map[string]int
}

type legacyAnchoredLfoCandidate struct {
	blob               []byte
	lsidByIlfo         map[int]uint32
	startByIlfoLevel   map[string]int
	formatByIlfoLevel  map[string]string
	restartByIlfoLevel map[string]int
	restartOnLevel     map[string]int
	score              int
	distance           int
	exact              bool
}

type legacyAnchoredLstCandidate struct {
	blob     []byte
	levels   map[uint32]legacyListLevelData
	score    int
	distance int
	exact    bool
}

const legacyFibFcLcbPlfLfoIndex = 20
const legacyFibFcLcbPlfLstIndex = 21
const legacyListOverrideClear = -1
const legacyStructuredListAnchorScanBack = 64
const legacyStructuredListAnchorScanAhead = 256

const legacyLfoSize = 16
const legacyLfolvlSize = 8
const legacyLstfSize = 28
const legacyLvlfSize = 28
const legacyLSTFFlagSimpleList = 1 << 0
const legacyLVLFFlagNoRestart = 1 << 3

func (t *legacyListTables) formatFor(ilfo, level int) string {
	if t == nil || len(t.formatByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return ""
	}
	return t.formatByIlfoLevel[legacyListKey(ilfo, level)]
}

func (t *legacyListTables) startFor(ilfo, level int) int {
	if t == nil || len(t.startByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return 0
	}
	return t.startByIlfoLevel[legacyListKey(ilfo, level)]
}

func (t *legacyListTables) restartFor(ilfo, level int) bool {
	if t == nil || len(t.restartByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return false
	}
	return t.restartByIlfoLevel[legacyListKey(ilfo, level)]
}

func (t *legacyListTables) restartOnLevelFor(ilfo, level int) (int, bool) {
	if t == nil || len(t.restartOnLevelByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return 0, false
	}
	val, ok := t.restartOnLevelByIlfoLevel[legacyListKey(ilfo, level)]
	return val, ok
}

func (t *legacyListTables) indentLeftFor(ilfo, level int) (int, bool) {
	if t == nil || len(t.indentLeftByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return 0, false
	}
	val, ok := t.indentLeftByIlfoLevel[legacyListKey(ilfo, level)]
	return val, ok
}

func (t *legacyListTables) indentFirstLineFor(ilfo, level int) (int, bool) {
	if t == nil || len(t.indentFirstLineByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return 0, false
	}
	val, ok := t.indentFirstLineByIlfoLevel[legacyListKey(ilfo, level)]
	return val, ok
}

func (t *legacyListTables) templateFor(ilfo, level int) string {
	if t == nil || len(t.templateByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return ""
	}
	return t.templateByIlfoLevel[legacyListKey(ilfo, level)]
}

func (t *legacyListTables) abstractNumIDFor(ilfo int) int {
	if t == nil || len(t.lsidByIlfo) == 0 || ilfo <= 0 {
		return 0
	}
	if lsid, ok := t.lsidByIlfo[ilfo]; ok {
		return int(lsid)
	}
	return 0
}

func (t *legacyListTables) runFormatFor(ilfo, level int) *wordir.RunFormat {
	if t == nil || len(t.runFormatByIlfoLevel) == 0 || ilfo <= 0 || level < 0 {
		return nil
	}
	return t.runFormatByIlfoLevel[legacyListKey(ilfo, level)]
}

func (t *legacyListTables) overrideFor(ilfo, level int) *wordir.LegacyNumberingOverride {
	if t == nil || ilfo <= 0 || level < 0 {
		return nil
	}
	key := legacyListKey(ilfo, level)
	var out wordir.LegacyNumberingOverride
	has := false
	if value, ok := t.overrideStartByIlfoLevel[key]; ok {
		v := value
		out.Start = &v
		has = true
	}
	if value, ok := t.overrideFormatByIlfoLevel[key]; ok {
		out.Format = value
		has = true
	}
	if value, ok := t.overrideRestartByIlfoLevel[key]; ok {
		v := value
		out.Restart = &v
		has = true
	}
	if value, ok := t.overrideRestartOnLevelByIlfoLevel[key]; ok {
		v := value
		out.RestartOnLevel = &v
		has = true
	}
	if !has {
		return nil
	}
	return &out
}

func legacyListKey(ilfo, level int) string {
	return strconv.Itoa(ilfo) + ":" + strconv.Itoa(level)
}

// loadLegacyListTables performs a minimal, best-effort decode of list metadata
// from table-stream blobs referenced by FibRgFcLcb97. The decoder intentionally
// accepts simplified PlfLfo/PlfLst-like layouts to improve list format fidelity
// when PAPx supplies ilfo/ilvl pointers.
func loadLegacyListTables(wordStream, tableStream []byte, fibBase, cbRgFcLcb int, papxEntries ...[]papxEntry) *legacyListTables {
	observed := map[int]struct{}{}
	observedLevels := map[int]map[int]struct{}{}
	if len(papxEntries) > 0 {
		observed = legacyObservedBinaryListOverrides(papxEntries[0])
		observedLevels = legacyObservedBinaryListLevels(papxEntries[0])
	}
	if len(observed) == 0 {
		return legacyLoadLegacyListTablesWithoutObserved(wordStream, tableStream, fibBase, cbRgFcLcb)
	}

	var lfoCandidates []legacyAnchoredLfoCandidate
	var lstCandidates []legacyAnchoredLstCandidate
	addLfoCandidate := func(candidate legacyAnchoredLfoCandidate, ok bool) {
		if !ok || !legacyLfoCandidatePlausible(candidate, observed) {
			return
		}
		lfoCandidates = append(lfoCandidates, candidate)
	}
	addLstCandidate := func(candidate legacyAnchoredLstCandidate, ok bool) {
		if !ok || len(candidate.levels) == 0 || candidate.score == 0 {
			return
		}
		lstCandidates = append(lstCandidates, candidate)
	}

	if blob, ok := legacyFibBlob(wordStream, tableStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLfoIndex); ok {
		addLfoCandidate(legacyDecodeLfoCandidate(blob, true, 0), true)
	}
	if candidate, ok := legacyFibLstCandidate(tableStream, wordStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLstIndex); ok {
		addLstCandidate(candidate, true)
	}
	if anchoredLfo, anchoredLst := legacyAnchoredListPair(wordStream, tableStream, fibBase, cbRgFcLcb); anchoredLfo != nil || anchoredLst != nil {
		if anchoredLfo != nil {
			addLfoCandidate(*anchoredLfo, true)
		}
		if anchoredLst != nil {
			addLstCandidate(*anchoredLst, true)
		}
	}
	for index := 0; index < cbRgFcLcb; index++ {
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			continue
		}
		if blob, ok := legacyFibBlob(wordStream, tableStream, fibBase, cbRgFcLcb, index); ok {
			addLfoCandidate(legacyDecodeLfoCandidate(blob, true, 0), true)
		}
		if candidate, ok := legacyFibLstCandidate(tableStream, wordStream, fibBase, cbRgFcLcb, index); ok {
			addLstCandidate(candidate, true)
		}
	}

	bestLfo, bestLst := legacySelectListCandidates(lfoCandidates, lstCandidates, observed)
	if bestLfo == nil || bestLst == nil {
		return nil
	}
	legacyReconcileObservedOrderedListMappings(bestLfo, bestLst, observedLevels)

	formatByIlfoLevel := map[string]string{}
	startByIlfoLevel := map[string]int{}
	restartByIlfoLevel := map[string]bool{}
	restartOnLevelByIlfoLevel := map[string]int{}
	indentLeftByIlfoLevel := map[string]int{}
	indentFirstLineByIlfoLevel := map[string]int{}
	templateByIlfoLevel := map[string]string{}
	runFormatByIlfoLevel := map[string]*wordir.RunFormat{}
	for ilfo, lsid := range bestLfo.lsidByIlfo {
		levels, ok := bestLst.levels[lsid]
		if !ok {
			continue
		}
		for level := range levels.format {
			key := legacyListKey(ilfo, level)
			template := ""
			if level < len(levels.template) {
				template = levels.template[level]
			}
			format := bestLfo.formatByIlfoLevel[key]
			if format == "" {
				format = legacyNfcToListFormat(levels.format[level])
			}
			format = legacyListFormatForTemplate(format, template)
			if format == "" {
				continue
			}
			formatByIlfoLevel[key] = format
			if bestLst.exact && level < len(levels.start) && levels.start[level] > 0 {
				startByIlfoLevel[key] = levels.start[level]
			}
			if overrideStart, ok := bestLfo.startByIlfoLevel[key]; ok {
				if overrideStart == legacyListOverrideClear {
					delete(startByIlfoLevel, key)
				} else if overrideStart > 0 {
					startByIlfoLevel[key] = overrideStart
				}
			}
			if bestLst.exact && level < len(levels.restart) && levels.restart[level] {
				restartByIlfoLevel[key] = true
			}
			if overrideRestart, ok := bestLfo.restartByIlfoLevel[key]; ok {
				if overrideRestart == legacyListOverrideClear {
					delete(restartByIlfoLevel, key)
				} else if overrideRestart > 0 {
					restartByIlfoLevel[key] = true
				}
			}
			if bestLst.exact && level < len(levels.restartOnLevel) && levels.restartOnLevel[level] >= 0 {
				restartOnLevelByIlfoLevel[key] = levels.restartOnLevel[level]
			}
			if overrideRestartOnLevel, ok := bestLfo.restartOnLevel[key]; ok {
				if overrideRestartOnLevel == legacyListOverrideClear {
					delete(restartOnLevelByIlfoLevel, key)
				} else {
					restartOnLevelByIlfoLevel[key] = overrideRestartOnLevel
				}
			}
			if bestLst.exact && level < len(levels.indentLeft) && levels.indentLeft[level] != 0 {
				indentLeftByIlfoLevel[key] = levels.indentLeft[level]
			}
			if bestLst.exact && level < len(levels.indentFirstLine) && levels.indentFirstLine[level] != 0 {
				indentFirstLineByIlfoLevel[key] = levels.indentFirstLine[level]
			}
			if bestLst.exact && level < len(levels.template) && levels.template[level] != "" {
				templateByIlfoLevel[key] = levels.template[level]
			}
			if bestLst.exact && level < len(levels.hasRunFormat) && levels.hasRunFormat[level] {
				rf := levels.runFormat[level]
				runFormatByIlfoLevel[key] = &rf
			}
		}
	}
	if len(formatByIlfoLevel) == 0 {
		return nil
	}
	overrideRestart := map[string]bool{}
	for key, value := range bestLfo.restartByIlfoLevel {
		overrideRestart[key] = value > 0
	}
	overrideRestartOnLevel := map[string]int{}
	for key, value := range bestLfo.restartOnLevel {
		overrideRestartOnLevel[key] = value
	}
	overrideStart := map[string]int{}
	for key, value := range bestLfo.startByIlfoLevel {
		if value > 0 {
			overrideStart[key] = value
		}
	}
	return &legacyListTables{
		formatByIlfoLevel:                 formatByIlfoLevel,
		startByIlfoLevel:                  startByIlfoLevel,
		restartByIlfoLevel:                restartByIlfoLevel,
		restartOnLevelByIlfoLevel:         restartOnLevelByIlfoLevel,
		indentLeftByIlfoLevel:             indentLeftByIlfoLevel,
		indentFirstLineByIlfoLevel:        indentFirstLineByIlfoLevel,
		templateByIlfoLevel:               templateByIlfoLevel,
		lsidByIlfo:                        bestLfo.lsidByIlfo,
		runFormatByIlfoLevel:              runFormatByIlfoLevel,
		overrideStartByIlfoLevel:          overrideStart,
		overrideFormatByIlfoLevel:         bestLfo.formatByIlfoLevel,
		overrideRestartByIlfoLevel:        overrideRestart,
		overrideRestartOnLevelByIlfoLevel: overrideRestartOnLevel,
	}
}

func legacyLoadLegacyListTablesWithoutObserved(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) *legacyListTables {
	var bestLfo map[int]uint32
	var bestLfoStarts map[string]int
	var bestLfoFormats map[string]string
	var bestLfoRestarts map[string]int
	var bestLfoRestartOnLevel map[string]int
	bestLfoExact := false
	bestLfoScore := 0
	bestLfoBlobLen := 0
	var bestLst map[uint32]legacyListLevelData
	bestLstExact := false
	bestLstScore := 0
	bestLstBlobLen := 0
	var anchoredLfo *legacyAnchoredLfoCandidate
	var anchoredLst *legacyAnchoredLstCandidate
	if blob, ok := legacyFibBlob(wordStream, tableStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLfoIndex); ok {
		bestLfo, bestLfoStarts, bestLfoFormats, bestLfoRestarts, bestLfoRestartOnLevel, bestLfoScore = decodeLegacyPlfLfoData(blob)
		bestLfoExact = bestLfoScore > 0
		bestLfoBlobLen = len(blob)
	}
	if blob, ok := legacyFibPlfLstBlob(wordStream, tableStream, fibBase, cbRgFcLcb); ok {
		bestLst = parseLegacyPlfLstBlob(blob)
		bestLstScore = legacyPlfLstScore(bestLst)
		bestLstExact = bestLstScore > 0
		bestLstBlobLen = len(blob)
		if bestLstScore == 0 {
			bestLst = nil
			bestLstBlobLen = 0
		}
	}
	if bestLfoScore == 0 || bestLstScore == 0 {
		anchoredLfo, anchoredLst = legacyAnchoredListPair(wordStream, tableStream, fibBase, cbRgFcLcb)
		if bestLfoScore == 0 && anchoredLfo != nil {
			bestLfo = anchoredLfo.lsidByIlfo
			bestLfoStarts = anchoredLfo.startByIlfoLevel
			bestLfoFormats = anchoredLfo.formatByIlfoLevel
			bestLfoRestarts = anchoredLfo.restartByIlfoLevel
			bestLfoRestartOnLevel = anchoredLfo.restartOnLevel
			bestLfoExact = true
			bestLfoScore = anchoredLfo.score
			bestLfoBlobLen = len(anchoredLfo.blob)
		}
		if bestLstScore == 0 && anchoredLst != nil {
			bestLst = anchoredLst.levels
			bestLstExact = true
			bestLstScore = anchoredLst.score
			bestLstBlobLen = len(anchoredLst.blob)
		}
	}
	for index := 0; index < cbRgFcLcb; index++ {
		if index == legacyFibFcLcbPlfLfoIndex || index == legacyFibFcLcbPlfLstIndex {
			continue
		}
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			continue
		}
		blob := tableStream[fc : fc+lcb]
		if bestLfoScore == 0 {
			lfo, starts, formats, restarts, restartOnLevel, score := decodeLegacyPlfLfoData(blob)
			if score > bestLfoScore || (score == bestLfoScore && score > 0 && len(blob) > bestLfoBlobLen) {
				bestLfo = lfo
				bestLfoStarts = starts
				bestLfoFormats = formats
				bestLfoRestarts = restarts
				bestLfoRestartOnLevel = restartOnLevel
				bestLfoExact = false
				bestLfoScore = score
				bestLfoBlobLen = len(blob)
			}
		}
		if bestLstScore == 0 {
			if lst := parseLegacyPlfLstBlob(blob); len(lst) > 0 {
				score := legacyPlfLstScore(lst)
				if score > bestLstScore || (score == bestLstScore && score > 0 && len(blob) > bestLstBlobLen) {
					bestLst = lst
					bestLstExact = false
					bestLstScore = score
					bestLstBlobLen = len(blob)
				}
			}
		}
	}
	if len(bestLfo) == 0 || len(bestLst) == 0 {
		return nil
	}

	formatByIlfoLevel := map[string]string{}
	startByIlfoLevel := map[string]int{}
	restartByIlfoLevel := map[string]bool{}
	restartOnLevelByIlfoLevel := map[string]int{}
	indentLeftByIlfoLevel := map[string]int{}
	indentFirstLineByIlfoLevel := map[string]int{}
	templateByIlfoLevel := map[string]string{}
	runFormatByIlfoLevel := map[string]*wordir.RunFormat{}
	for ilfo, lsid := range bestLfo {
		levels, ok := bestLst[lsid]
		if !ok {
			continue
		}
		for level := range levels.format {
			key := legacyListKey(ilfo, level)
			template := ""
			if level < len(levels.template) {
				template = levels.template[level]
			}
			format := bestLfoFormats[key]
			if format == "" {
				format = legacyNfcToListFormat(levels.format[level])
			}
			format = legacyListFormatForTemplate(format, template)
			if format == "" {
				continue
			}
			formatByIlfoLevel[key] = format
			if bestLstExact && level < len(levels.start) && levels.start[level] > 0 {
				startByIlfoLevel[key] = levels.start[level]
			}
			if bestLfoExact {
				if overrideStart, ok := bestLfoStarts[key]; ok {
					if overrideStart == legacyListOverrideClear {
						delete(startByIlfoLevel, key)
					} else if overrideStart > 0 {
						startByIlfoLevel[key] = overrideStart
					}
				}
			}
			if bestLstExact && level < len(levels.restart) && levels.restart[level] {
				restartByIlfoLevel[key] = true
			}
			if bestLfoExact {
				if overrideRestart, ok := bestLfoRestarts[key]; ok {
					if overrideRestart == legacyListOverrideClear {
						delete(restartByIlfoLevel, key)
					} else if overrideRestart > 0 {
						restartByIlfoLevel[key] = true
					}
				}
			}
			if bestLstExact && level < len(levels.restartOnLevel) && levels.restartOnLevel[level] >= 0 {
				restartOnLevelByIlfoLevel[key] = levels.restartOnLevel[level]
			}
			if bestLfoExact {
				if overrideRestartOnLevel, ok := bestLfoRestartOnLevel[key]; ok {
					if overrideRestartOnLevel == legacyListOverrideClear {
						delete(restartOnLevelByIlfoLevel, key)
					} else {
						restartOnLevelByIlfoLevel[key] = overrideRestartOnLevel
					}
				}
			}
			if bestLstExact && level < len(levels.indentLeft) && levels.indentLeft[level] != 0 {
				indentLeftByIlfoLevel[key] = levels.indentLeft[level]
			}
			if bestLstExact && level < len(levels.indentFirstLine) && levels.indentFirstLine[level] != 0 {
				indentFirstLineByIlfoLevel[key] = levels.indentFirstLine[level]
			}
			if bestLstExact && level < len(levels.template) && levels.template[level] != "" {
				templateByIlfoLevel[key] = levels.template[level]
			}
			if bestLstExact && level < len(levels.hasRunFormat) && levels.hasRunFormat[level] {
				rf := levels.runFormat[level]
				runFormatByIlfoLevel[key] = &rf
			}
		}
	}
	if len(formatByIlfoLevel) == 0 {
		return nil
	}
	return &legacyListTables{
		formatByIlfoLevel:          formatByIlfoLevel,
		startByIlfoLevel:           startByIlfoLevel,
		restartByIlfoLevel:         restartByIlfoLevel,
		restartOnLevelByIlfoLevel:  restartOnLevelByIlfoLevel,
		indentLeftByIlfoLevel:      indentLeftByIlfoLevel,
		indentFirstLineByIlfoLevel: indentFirstLineByIlfoLevel,
		templateByIlfoLevel:        templateByIlfoLevel,
		lsidByIlfo:                 bestLfo,
		runFormatByIlfoLevel:       runFormatByIlfoLevel,
	}
}

func legacyObservedBinaryListOverrides(entries []papxEntry) map[int]struct{} {
	observed := map[int]struct{}{}
	for _, entry := range entries {
		if entry.Format.BinaryListOverride == nil || *entry.Format.BinaryListOverride <= 0 {
			continue
		}
		observed[*entry.Format.BinaryListOverride] = struct{}{}
	}
	return observed
}

func legacyObservedBinaryListLevels(entries []papxEntry) map[int]map[int]struct{} {
	observed := map[int]map[int]struct{}{}
	for _, entry := range entries {
		if entry.Format.BinaryListOverride == nil || *entry.Format.BinaryListOverride <= 0 {
			continue
		}
		ilfo := *entry.Format.BinaryListOverride
		level := 0
		if entry.Format.BinaryListLevel != nil && *entry.Format.BinaryListLevel >= 0 {
			level = *entry.Format.BinaryListLevel
		}
		if observed[ilfo] == nil {
			observed[ilfo] = map[int]struct{}{}
		}
		observed[ilfo][level] = struct{}{}
	}
	return observed
}

func legacyDecodeLfoCandidate(blob []byte, exact bool, distance int) legacyAnchoredLfoCandidate {
	lsidByIlfo, starts, formats, restarts, restartOn, score := decodeLegacyPlfLfoData(blob)
	return legacyAnchoredLfoCandidate{
		blob:               blob,
		lsidByIlfo:         lsidByIlfo,
		startByIlfoLevel:   starts,
		formatByIlfoLevel:  formats,
		restartByIlfoLevel: restarts,
		restartOnLevel:     restartOn,
		score:              score,
		distance:           distance,
		exact:              exact,
	}
}

func legacyFibLstCandidate(tableStream, wordStream []byte, fibBase, cbRgFcLcb, index int) (legacyAnchoredLstCandidate, bool) {
	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
	if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
		return legacyAnchoredLstCandidate{}, false
	}
	tail := tableStream[fc:]
	if full, ok := legacyExtendStructuredPlfLstBlob(tail, lcb); ok {
		levels := parseLegacyPlfLstBlob(full)
		score := legacyPlfLstScore(levels)
		if score > 0 {
			return legacyAnchoredLstCandidate{blob: full, levels: levels, score: score, exact: true}, true
		}
	}
	blob := tableStream[fc : fc+lcb]
	levels := parseLegacyPlfLstBlob(blob)
	score := legacyPlfLstScore(levels)
	if score == 0 {
		return legacyAnchoredLstCandidate{}, false
	}
	return legacyAnchoredLstCandidate{blob: blob, levels: levels, score: score}, true
}

func legacyLfoCandidatePlausible(candidate legacyAnchoredLfoCandidate, observed map[int]struct{}) bool {
	if len(candidate.lsidByIlfo) == 0 || candidate.score == 0 {
		return false
	}
	if len(observed) == 0 {
		return len(candidate.lsidByIlfo) <= 32
	}
	hit := 0
	for ilfo := range observed {
		if _, ok := candidate.lsidByIlfo[ilfo]; ok {
			hit++
		}
	}
	extra := len(candidate.lsidByIlfo) - hit
	return hit > 0 && extra <= hit+4
}

func legacyLfoCandidateRank(candidate legacyAnchoredLfoCandidate, observed map[int]struct{}) int {
	rank := candidate.score
	if candidate.exact {
		rank += 1000
	}
	if len(observed) == 0 {
		return rank - len(candidate.lsidByIlfo)*10
	}
	hit := 0
	for ilfo := range observed {
		if _, ok := candidate.lsidByIlfo[ilfo]; ok {
			hit++
		}
	}
	extra := len(candidate.lsidByIlfo) - hit
	return rank + hit*10000 - extra*500
}

func legacyLstCandidateRank(candidate legacyAnchoredLstCandidate) int {
	rank := candidate.score
	if candidate.exact {
		rank += 1000
	}
	return rank - len(candidate.levels)*10
}

func legacySelectListCandidates(lfoCandidates []legacyAnchoredLfoCandidate, lstCandidates []legacyAnchoredLstCandidate, observed map[int]struct{}) (*legacyAnchoredLfoCandidate, *legacyAnchoredLstCandidate) {
	if len(observed) == 0 {
		var bestLfo *legacyAnchoredLfoCandidate
		bestLfoRank := 0
		for i := range lfoCandidates {
			rank := lfoCandidates[i].score
			if lfoCandidates[i].exact {
				rank += 1000
			}
			if bestLfo == nil || rank > bestLfoRank {
				bestLfo = &lfoCandidates[i]
				bestLfoRank = rank
			}
		}
		var bestLst *legacyAnchoredLstCandidate
		bestLstRank := 0
		for i := range lstCandidates {
			rank := lstCandidates[i].score
			if lstCandidates[i].exact {
				rank += 1000
			}
			if bestLst == nil || rank > bestLstRank {
				bestLst = &lstCandidates[i]
				bestLstRank = rank
			}
		}
		return bestLfo, bestLst
	}

	var bestLfo *legacyAnchoredLfoCandidate
	var bestLst *legacyAnchoredLstCandidate
	bestPairRank := 0
	for i := range lfoCandidates {
		lfoRank := legacyLfoCandidateRank(lfoCandidates[i], observed)
		for j := range lstCandidates {
			overlap := legacyAnchoredListOverlap(lfoCandidates[i].lsidByIlfo, lstCandidates[j].levels)
			if overlap == 0 {
				continue
			}
			pairRank := overlap*10000 + lfoRank + legacyLstCandidateRank(lstCandidates[j])
			if bestLfo == nil || pairRank > bestPairRank {
				bestLfo = &lfoCandidates[i]
				bestLst = &lstCandidates[j]
				bestPairRank = pairRank
			}
		}
	}
	if bestLfo != nil && bestLst != nil {
		return bestLfo, bestLst
	}

	bestLfoRank := 0
	for i := range lfoCandidates {
		rank := legacyLfoCandidateRank(lfoCandidates[i], observed)
		if bestLfo == nil || rank > bestLfoRank {
			bestLfo = &lfoCandidates[i]
			bestLfoRank = rank
		}
	}
	bestLstRank := 0
	for i := range lstCandidates {
		rank := legacyLstCandidateRank(lstCandidates[i])
		if bestLst == nil || rank > bestLstRank {
			bestLst = &lstCandidates[i]
			bestLstRank = rank
		}
	}
	return bestLfo, bestLst
}

type legacyObservedOrderedIlfo struct {
	ilfo  int
	score int
}

type legacyOrderedListProfile struct {
	lsid  uint32
	score int
}

func legacyReconcileObservedOrderedListMappings(bestLfo *legacyAnchoredLfoCandidate, bestLst *legacyAnchoredLstCandidate, observedLevels map[int]map[int]struct{}) {
	if bestLfo == nil || bestLst == nil || !bestLfo.exact {
		return
	}
	if len(bestLfo.startByIlfoLevel) > 0 || len(bestLfo.formatByIlfoLevel) > 0 || len(bestLfo.restartByIlfoLevel) > 0 || len(bestLfo.restartOnLevel) > 0 {
		return
	}

	var orderedIlfos []legacyObservedOrderedIlfo
	for ilfo, levels := range observedLevels {
		lsid, ok := bestLfo.lsidByIlfo[ilfo]
		if !ok {
			continue
		}
		levelData, ok := bestLst.levels[lsid]
		if !ok || legacyListLevelDataIsBullet(levelData) {
			continue
		}
		score := 0
		for level := range levels {
			score += (level + 1) * 100
		}
		orderedIlfos = append(orderedIlfos, legacyObservedOrderedIlfo{ilfo: ilfo, score: score})
	}
	if len(orderedIlfos) < 2 {
		return
	}

	var orderedProfiles []legacyOrderedListProfile
	for lsid, levelData := range bestLst.levels {
		if legacyListLevelDataIsBullet(levelData) {
			continue
		}
		orderedProfiles = append(orderedProfiles, legacyOrderedListProfile{lsid: lsid, score: legacyOrderedListProfileScore(levelData)})
	}
	if len(orderedProfiles) != len(orderedIlfos) {
		return
	}

	sort.Slice(orderedIlfos, func(i, j int) bool { return orderedIlfos[i].score > orderedIlfos[j].score })
	sort.Slice(orderedProfiles, func(i, j int) bool { return orderedProfiles[i].score > orderedProfiles[j].score })
	for i := range orderedIlfos {
		bestLfo.lsidByIlfo[orderedIlfos[i].ilfo] = orderedProfiles[i].lsid
	}
}

func legacyListLevelDataIsBullet(levelData legacyListLevelData) bool {
	if len(levelData.format) == 0 {
		return false
	}
	for _, format := range levelData.format {
		if format != 23 {
			return false
		}
	}
	return true
}

func legacyOrderedListProfileScore(levelData legacyListLevelData) int {
	// Prefer profiles whose actual number-text templates reference ancestor
	// levels. That is the strongest signal that an observed deep list belongs
	// to a true compound numbering definition ("%1.%2.%3.") rather than a
	// single-level style cycle (decimal/lower-alpha/lower-roman).
	//
	// Older/synthetic list tables may not carry templates at all; retain the
	// previous format-diversity score as a fallback for those files.
	templateScore := 0
	for level, template := range levelData.template {
		refs := legacyTemplateReferencedLevelCount(template)
		if refs > 1 {
			// Weight compound semantics above format diversity and reward deeper
			// templates so a four-level observed list maps to a four-level profile.
			templateScore += refs * 10000
			if level+1 > refs {
				templateScore += level + 1
			} else {
				templateScore += refs
			}
		}
	}
	if templateScore > 0 {
		return 100000 + templateScore
	}

	seen := map[byte]struct{}{}
	maxLevel := 0
	for level, format := range levelData.format {
		if format == 23 {
			continue
		}
		seen[format] = struct{}{}
		if level > maxLevel {
			maxLevel = level
		}
	}
	return len(seen)*1000 + maxLevel
}

func legacyTemplateReferencedLevelCount(template string) int {
	seen := map[byte]struct{}{}
	for i := 0; i+1 < len(template); i++ {
		if template[i] != '%' {
			continue
		}
		level := template[i+1]
		if level >= '1' && level <= '9' {
			seen[level] = struct{}{}
			i++
		}
	}
	return len(seen)
}

func legacyAnchoredListPair(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) (*legacyAnchoredLfoCandidate, *legacyAnchoredLstCandidate) {
	lfoFC, lfoLCB, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLfoIndex)
	if !ok || lfoFC <= 0 || lfoFC >= len(tableStream) {
		return nil, nil
	}
	lstFC, lstLCB, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLstIndex)
	if !ok || lstFC <= 0 || lstFC >= len(tableStream) {
		return nil, nil
	}
	_ = lfoLCB
	_ = lstLCB
	lfoCandidates := legacyAnchoredLfoCandidates(tableStream, lfoFC)
	lstCandidates := legacyAnchoredLstCandidates(tableStream, lstFC)
	if len(lfoCandidates) == 0 || len(lstCandidates) == 0 {
		return nil, nil
	}
	var bestLfo *legacyAnchoredLfoCandidate
	var bestLst *legacyAnchoredLstCandidate
	bestOverlap := 0
	bestScore := 0
	bestDistance := 0
	for i := range lfoCandidates {
		for j := range lstCandidates {
			overlap := legacyAnchoredListOverlap(lfoCandidates[i].lsidByIlfo, lstCandidates[j].levels)
			if overlap == 0 {
				continue
			}
			score := overlap*10000 + lfoCandidates[i].score + lstCandidates[j].score
			distance := lfoCandidates[i].distance + lstCandidates[j].distance
			if bestLfo == nil || overlap > bestOverlap || (overlap == bestOverlap && score > bestScore) || (overlap == bestOverlap && score == bestScore && distance < bestDistance) {
				bestLfo = &lfoCandidates[i]
				bestLst = &lstCandidates[j]
				bestOverlap = overlap
				bestScore = score
				bestDistance = distance
			}
		}
	}
	return bestLfo, bestLst
}

func legacyAnchoredLfoCandidates(tableStream []byte, fc int) []legacyAnchoredLfoCandidate {
	start := fc - legacyStructuredListAnchorScanBack
	if start < 0 {
		start = 0
	}
	end := fc + legacyStructuredListAnchorScanAhead
	if end > len(tableStream) {
		end = len(tableStream)
	}
	var out []legacyAnchoredLfoCandidate
	for off := start; off < end; off++ {
		distance := off - fc
		if distance < 0 {
			distance = -distance
		}
		if lsidByIlfo, starts, formats, restarts, restartOn, score, size, ok := parseLegacyStructuredPlfLfoPrefix(tableStream[off:]); ok && score > 0 {
			out = append(out, legacyAnchoredLfoCandidate{
				blob:               tableStream[off : off+size],
				lsidByIlfo:         lsidByIlfo,
				startByIlfoLevel:   starts,
				formatByIlfoLevel:  formats,
				restartByIlfoLevel: restarts,
				restartOnLevel:     restartOn,
				score:              score + 1000,
				distance:           distance,
				exact:              true,
			})
			continue
		}
		lsidByIlfo, starts, formats, restarts, restartOn, score := decodeLegacyPlfLfoData(tableStream[off:])
		if score == 0 {
			continue
		}
		out = append(out, legacyAnchoredLfoCandidate{
			blob:               tableStream[off:],
			lsidByIlfo:         lsidByIlfo,
			startByIlfoLevel:   starts,
			formatByIlfoLevel:  formats,
			restartByIlfoLevel: restarts,
			restartOnLevel:     restartOn,
			score:              score,
			distance:           distance,
			exact:              false,
		})
	}
	return out
}

func legacyAnchoredLstCandidates(tableStream []byte, fc int) []legacyAnchoredLstCandidate {
	start := fc - legacyStructuredListAnchorScanBack
	if start < 0 {
		start = 0
	}
	end := fc + legacyStructuredListAnchorScanAhead
	if end > len(tableStream) {
		end = len(tableStream)
	}
	var out []legacyAnchoredLstCandidate
	for off := start; off < end; off++ {
		distance := off - fc
		if distance < 0 {
			distance = -distance
		}
		if levels, score, size, ok := parseLegacyStructuredPlfLstPrefix(tableStream[off:]); ok && score > 0 {
			out = append(out, legacyAnchoredLstCandidate{
				blob:     tableStream[off : off+size],
				levels:   levels,
				score:    score + 1000,
				distance: distance,
				exact:    true,
			})
			continue
		}
		levels := parseLegacyPlfLstBlob(tableStream[off:])
		score := legacyPlfLstScore(levels)
		if score == 0 {
			continue
		}
		out = append(out, legacyAnchoredLstCandidate{
			blob:     tableStream[off:],
			levels:   levels,
			score:    score,
			distance: distance,
			exact:    false,
		})
	}
	return out
}

func legacyAnchoredListOverlap(lsidByIlfo map[int]uint32, levels map[uint32]legacyListLevelData) int {
	if len(lsidByIlfo) == 0 || len(levels) == 0 {
		return 0
	}
	overlap := 0
	for _, lsid := range lsidByIlfo {
		if _, ok := levels[lsid]; ok {
			overlap++
		}
	}
	return overlap
}

func legacyAnchoredStructuredPlfLfoBlob(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) ([]byte, bool) {
	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLfoIndex)
	if !ok || fc <= 0 || fc >= len(tableStream) {
		return nil, false
	}
	_ = lcb
	start := fc - legacyStructuredListAnchorScanBack
	if start < 0 {
		start = 0
	}
	end := fc + legacyStructuredListAnchorScanAhead
	if end > len(tableStream) {
		end = len(tableStream)
	}
	bestOffset := -1
	bestScore := 0
	bestSize := 0
	bestDistance := 0
	for off := start; off < end; off++ {
		score := 0
		size := 0
		exact := false
		if _, _, _, _, _, structuredScore, structuredSize, ok := parseLegacyStructuredPlfLfoPrefix(tableStream[off:]); ok && structuredScore > 0 {
			score = structuredScore + 1000
			size = structuredSize
			exact = true
		} else {
			_, _, _, _, _, decodedScore := decodeLegacyPlfLfoData(tableStream[off:])
			if decodedScore == 0 {
				continue
			}
			score = decodedScore
			size = len(tableStream) - off
		}
		distance := off - fc
		if distance < 0 {
			distance = -distance
		}
		if bestOffset < 0 || score > bestScore || (score == bestScore && distance < bestDistance) {
			bestOffset = off
			bestScore = score
			bestSize = size
			bestDistance = distance
			_ = exact
		}
	}
	if bestOffset < 0 {
		return nil, false
	}
	return tableStream[bestOffset : bestOffset+bestSize], true
}

func legacyAnchoredStructuredPlfLstBlob(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) ([]byte, bool) {
	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLstIndex)
	if !ok || fc <= 0 || fc >= len(tableStream) {
		return nil, false
	}
	_ = lcb
	start := fc - legacyStructuredListAnchorScanBack
	if start < 0 {
		start = 0
	}
	end := fc + legacyStructuredListAnchorScanAhead
	if end > len(tableStream) {
		end = len(tableStream)
	}
	bestOffset := -1
	bestScore := 0
	bestSize := 0
	bestDistance := 0
	for off := start; off < end; off++ {
		score := 0
		size := 0
		if _, structuredScore, structuredSize, ok := parseLegacyStructuredPlfLstPrefix(tableStream[off:]); ok && structuredScore > 0 {
			score = structuredScore + 1000
			size = structuredSize
		} else if levels := parseLegacyPlfLstBlob(tableStream[off:]); len(levels) > 0 {
			decodedScore := legacyPlfLstScore(levels)
			if decodedScore == 0 {
				continue
			}
			score = decodedScore
			size = len(tableStream) - off
		} else {
			continue
		}
		distance := off - fc
		if distance < 0 {
			distance = -distance
		}
		if bestOffset < 0 || score > bestScore || (score == bestScore && distance < bestDistance) {
			bestOffset = off
			bestScore = score
			bestSize = size
			bestDistance = distance
		}
	}
	if bestOffset < 0 {
		return nil, false
	}
	return tableStream[bestOffset : bestOffset+bestSize], true
}

func legacyFibBlob(wordStream, tableStream []byte, fibBase, cbRgFcLcb, index int) ([]byte, bool) {
	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
	if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
		return nil, false
	}
	return tableStream[fc : fc+lcb], true
}

func legacyFibPlfLstBlob(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) ([]byte, bool) {
	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, legacyFibFcLcbPlfLstIndex)
	if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
		return nil, false
	}
	tail := tableStream[fc:]
	if full, ok := legacyExtendStructuredPlfLstBlob(tail, lcb); ok {
		return full, true
	}
	return tableStream[fc : fc+lcb], true
}

func legacyExtendStructuredPlfLstBlob(blob []byte, declaredLen int) ([]byte, bool) {
	if declaredLen < 2 || declaredLen > len(blob) {
		return nil, false
	}
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	if count <= 0 || count > 128 {
		return nil, false
	}
	baseLen := 2 + count*legacyLstfSize
	if declaredLen < baseLen || baseLen > len(blob) {
		return nil, false
	}
	off := baseLen
	for i := 0; i < count; i++ {
		lstfOff := 2 + i*legacyLstfSize
		levelCount := 9
		if blob[lstfOff+26]&legacyLSTFFlagSimpleList != 0 {
			levelCount = 1
		}
		for level := 0; level < levelCount; level++ {
			_, size, ok := parseLegacyStructuredLVL(blob[off:])
			if !ok {
				return nil, false
			}
			off += size
			if off > len(blob) {
				return nil, false
			}
		}
	}
	return blob[:off], true
}

func decodeLegacyPlfLfoData(blob []byte) (map[int]uint32, map[string]int, map[string]string, map[string]int, map[string]int, int) {
	if lsidByIlfo, startByIlfoLevel, formatByIlfoLevel, restartByIlfoLevel, restartOnLevelByIlfoLevel, ok := parseLegacyStructuredPlfLfoData(blob); ok {
		score := len(lsidByIlfo)*10 + len(startByIlfoLevel) + len(formatByIlfoLevel) + len(restartByIlfoLevel) + len(restartOnLevelByIlfoLevel)
		return lsidByIlfo, startByIlfoLevel, formatByIlfoLevel, restartByIlfoLevel, restartOnLevelByIlfoLevel, score
	}
	lsidByIlfo := parseLegacyPlfLfoBlob(blob)
	startByIlfoLevel := parseLegacyPlfLfoStartOverrides(blob)
	formatByIlfoLevel := parseLegacyPlfLfoFormatOverrides(blob)
	restartByIlfoLevel := parseLegacyPlfLfoRestartOverrides(blob)
	restartOnLevelByIlfoLevel := parseLegacyPlfLfoRestartOnLevelOverrides(blob)
	score := len(lsidByIlfo)*10 + len(startByIlfoLevel) + len(formatByIlfoLevel) + len(restartByIlfoLevel) + len(restartOnLevelByIlfoLevel)
	return lsidByIlfo, startByIlfoLevel, formatByIlfoLevel, restartByIlfoLevel, restartOnLevelByIlfoLevel, score
}

type legacyListLevelData struct {
	format          []byte
	start           []int
	restart         []bool
	restartOnLevel  []int
	indentLeft      []int    // per-level left indent from LVLF PAPx grpprl (0 = unset)
	indentFirstLine []int    // per-level first-line/hanging indent from LVLF PAPx grpprl (0 = unset)
	template        []string // per-level LVLF xst template with %1, %2, ... placeholders
	hasRunFormat    []bool
	runFormat       []wordir.RunFormat // per-level character formatting from LVLF CHPx grpprl
}

type legacyStructuredLevelInfo struct {
	format            byte
	start             int
	hasRestart        bool
	restart           bool
	hasRestartOnLevel bool
	restartOnLevel    int
	indentLeft        int    // left indent from LVLF PAPx grpprl sprmPDxaLeft (0 = unset)
	indentFirstLine   int    // first-line indent from LVLF PAPx grpprl sprmPDxaLeft1 (0 = unset)
	template          string // LVLF xst with placeholders mapped to %1, %2, ... (1-indexed)
	hasRunFormat      bool
	runFormat         wordir.RunFormat // character formatting from LVLF CHPx grpprl
}

type legacyStructuredLevelOverride struct {
	level             int
	hasFormat         bool
	format            byte
	hasStart          bool
	start             int
	hasRestart        bool
	restart           bool
	hasRestartOnLevel bool
	restartOnLevel    int
}

func parseLegacyStructuredPlfLfoData(blob []byte) (map[int]uint32, map[string]int, map[string]string, map[string]int, map[string]int, bool) {
	lsidByIlfo, startByIlfoLevel, formatByIlfoLevel, restartByIlfoLevel, restartOnLevelByIlfoLevel, _, size, ok := parseLegacyStructuredPlfLfoPrefix(blob)
	if !ok || size != len(blob) {
		return nil, nil, nil, nil, nil, false
	}
	return lsidByIlfo, startByIlfoLevel, formatByIlfoLevel, restartByIlfoLevel, restartOnLevelByIlfoLevel, true
}

func parseLegacyStructuredPlfLfoPrefix(blob []byte) (map[int]uint32, map[string]int, map[string]string, map[string]int, map[string]int, int, int, bool) {
	if len(blob) < 4+legacyLfoSize {
		return nil, nil, nil, nil, nil, 0, 0, false
	}
	lfoMac := int(binary.LittleEndian.Uint32(blob[0:4]))
	if lfoMac <= 0 || lfoMac > 128 {
		return nil, nil, nil, nil, nil, 0, 0, false
	}
	lfoOffset := 4
	lfoDataOffset := lfoOffset + lfoMac*legacyLfoSize
	if lfoDataOffset+4*lfoMac > len(blob) {
		return nil, nil, nil, nil, nil, 0, 0, false
	}
	type legacyLfoEntry struct {
		lsid    uint32
		clfolvl int
	}
	entries := make([]legacyLfoEntry, lfoMac)
	lsidByIlfo := make(map[int]uint32, lfoMac)
	for i := 0; i < lfoMac; i++ {
		off := lfoOffset + i*legacyLfoSize
		lsid := binary.LittleEndian.Uint32(blob[off : off+4])
		entries[i] = legacyLfoEntry{lsid: lsid, clfolvl: int(blob[off+12])}
		if lsid != 0 {
			lsidByIlfo[i+1] = lsid
		}
	}
	if len(lsidByIlfo) == 0 {
		return nil, nil, nil, nil, nil, 0, 0, false
	}
	startByIlfoLevel := map[string]int{}
	formatByIlfoLevel := map[string]string{}
	restartByIlfoLevel := map[string]int{}
	restartOnLevelByIlfoLevel := map[string]int{}
	off := lfoDataOffset
	score := len(lsidByIlfo) * 10
	for i, entry := range entries {
		if off+4 > len(blob) {
			return nil, nil, nil, nil, nil, 0, 0, false
		}
		off += 4 // cp
		for overrideIndex := 0; overrideIndex < entry.clfolvl; overrideIndex++ {
			override, size, ok := parseLegacyStructuredLFOLVL(blob[off:])
			if !ok {
				return nil, nil, nil, nil, nil, 0, 0, false
			}
			off += size
			key := legacyListKey(i+1, override.level)
			if override.hasFormat {
				if format := legacyNfcToListFormat(override.format); format != "" {
					formatByIlfoLevel[key] = format
					score++
				}
			}
			if override.hasStart {
				startByIlfoLevel[key] = override.start
				score++
			}
			if override.hasRestart {
				if override.restart {
					restartByIlfoLevel[key] = 1
				} else {
					restartByIlfoLevel[key] = legacyListOverrideClear
				}
				score++
			}
			if override.hasRestartOnLevel {
				restartOnLevelByIlfoLevel[key] = override.restartOnLevel
				score++
			}
		}
	}
	return lsidByIlfo, startByIlfoLevel, formatByIlfoLevel, restartByIlfoLevel, restartOnLevelByIlfoLevel, score, off, true
}

func parseLegacyStructuredLFOLVL(blob []byte) (legacyStructuredLevelOverride, int, bool) {
	if len(blob) < legacyLfolvlSize {
		return legacyStructuredLevelOverride{}, 0, false
	}
	startAt := int(int32(binary.LittleEndian.Uint32(blob[0:4])))
	flags := binary.LittleEndian.Uint32(blob[4:8])
	level := int(flags & 0x0F)
	if level < 0 || level > 8 {
		return legacyStructuredLevelOverride{}, 0, false
	}
	override := legacyStructuredLevelOverride{level: level}
	fStartAt := flags&(1<<4) != 0
	fFormatting := flags&(1<<5) != 0
	if !fFormatting {
		if fStartAt && startAt > 0 {
			override.hasStart = true
			override.start = startAt
		}
		return override, legacyLfolvlSize, true
	}
	lvl, lvlSize, ok := parseLegacyStructuredLVL(blob[legacyLfolvlSize:])
	if !ok {
		return legacyStructuredLevelOverride{}, 0, false
	}
	override.hasFormat = true
	override.format = lvl.format
	if lvl.start > 0 {
		override.hasStart = true
		override.start = lvl.start
	}
	if lvl.hasRestartOnLevel {
		override.hasRestartOnLevel = true
		override.restartOnLevel = lvl.restartOnLevel
	}
	if lvl.hasRestart {
		override.hasRestart = true
		override.restart = lvl.restart
	} else if lvl.hasRestartOnLevel {
		// An LFO formatting override that carries only ilvlRestartLim still
		// clears the abstract level's explicit restart bit.
		override.hasRestart = true
		override.restart = false
	}
	if !lvl.hasRestart && !lvl.hasRestartOnLevel {
		override.hasRestart = true
		override.restart = false
		override.hasRestartOnLevel = true
		override.restartOnLevel = legacyListOverrideClear
	}
	return override, legacyLfolvlSize + lvlSize, true
}

func parseLegacyStructuredLVL(blob []byte) (legacyStructuredLevelInfo, int, bool) {
	if len(blob) < legacyLvlfSize {
		return legacyStructuredLevelInfo{}, 0, false
	}
	cbGrpprlChpx := int(blob[24])
	cbGrpprlPapx := int(blob[25])
	xstOffset := legacyLvlfSize + cbGrpprlPapx + cbGrpprlChpx
	if xstOffset > len(blob) {
		return legacyStructuredLevelInfo{}, 0, false
	}
	xstSize, ok := legacyXstSize(blob[xstOffset:])
	if !ok {
		return legacyStructuredLevelInfo{}, 0, false
	}
	size := xstOffset + xstSize
	if size > len(blob) {
		return legacyStructuredLevelInfo{}, 0, false
	}
	format := blob[4]
	info := legacyStructuredLevelInfo{format: format}
	// Extract paragraph indentation from LVLF PAPx grpprl bytes.
	// These SPRMs set the list-level indent defaults for paragraphs at this level.
	if cbGrpprlPapx > 0 && legacyLvlfSize+cbGrpprlPapx <= len(blob) {
		papxGrpprl := blob[legacyLvlfSize : legacyLvlfSize+cbGrpprlPapx]
		info.indentLeft, info.indentFirstLine = parseLVLFPapxIndents(papxGrpprl)
	}
	// Extract character formatting from LVLF CHPx grpprl bytes.
	// These SPRMs control the visual formatting of the list marker text.
	if cbGrpprlChpx > 0 {
		chpxStart := legacyLvlfSize + cbGrpprlPapx
		chpxEnd := chpxStart + cbGrpprlChpx
		if chpxEnd <= len(blob) {
			info.hasRunFormat = true
			info.runFormat = parseChpxGrpprl(blob[chpxStart:chpxEnd])
		}
	}
	// Decode the LVLF xst into a "%1.%2." template using rgbxchNums (offsets 6-14).
	if xstSize >= 2 && xstOffset+xstSize <= len(blob) {
		info.template = decodeLegacyLvlfTemplate(blob[0:legacyLvlfSize], blob[xstOffset:xstOffset+xstSize])
	}
	if legacyNfcToListFormat(format) == "" {
		return info, size, true
	}
	startAt := int(int32(binary.LittleEndian.Uint32(blob[0:4])))
	if startAt > 0 {
		info.start = startAt
	}
	flags := blob[5]
	if flags&legacyLVLFFlagNoRestart == 0 {
		info.hasRestart = true
		info.restart = true
		return info, size, true
	}
	if restartLimit := int(blob[26]); restartLimit > 0 && restartLimit <= 8 {
		info.hasRestartOnLevel = true
		info.restartOnLevel = restartLimit - 1
	}
	return info, size, true
}

// parseLVLFPapxIndents scans a LVLF PAPx grpprl byte sequence for the two
// indent-controlling SPRMs used to position list numbers/bullets:
//   - 0x840E  sprmPDxaLeft   — left indent in twips (signed)
//   - 0x8411  sprmPDxaLeft1  — first-line indent in twips (signed; negative = hanging)
//
// Returns (indentLeft, indentFirstLine); 0 means not found in the grpprl.
func parseLVLFPapxIndents(grpprl []byte) (indentLeft, indentFirstLine int) {
	pos := 0
	for pos+2 <= len(grpprl) {
		code := binary.LittleEndian.Uint16(grpprl[pos:])
		pos += 2
		size := chpxOperandSize(code, grpprl, pos)
		if size <= 0 || pos+size > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+size]
		pos += size
		switch code {
		case 0x840E: // sprmPDxaLeft — left indent
			if len(op) >= 2 {
				indentLeft = int(int16(binary.LittleEndian.Uint16(op)))
			}
		case 0x8411: // sprmPDxaLeft1 — first-line/hanging indent
			if len(op) >= 2 {
				indentFirstLine = int(int16(binary.LittleEndian.Uint16(op)))
			}
		}
	}
	return
}

func legacyXstSize(blob []byte) (int, bool) {
	if len(blob) < 2 {
		return 0, false
	}
	cch := int(binary.LittleEndian.Uint16(blob[0:2]))
	size := 2 + cch*2
	if size > len(blob) {
		return 0, false
	}
	return size, true
}

// decodeLegacyLvlfTemplate converts the LVLF rgbxchNums + xst pair into a
// "%1.%2." template string. rgbxchNums (LVLF bytes 6..14) carries 1-based
// indexes into the xst character data; the character at xst[index-1] is the
// placeholder for level (rgbxchNums slot index + 1). Non-placeholder runes
// are emitted verbatim. Returns "" when xst is empty or unparseable.
func decodeLegacyLvlfTemplate(lvlfHeader, xst []byte) string {
	if len(lvlfHeader) < legacyLvlfSize || len(xst) < 2 {
		return ""
	}
	cch := int(binary.LittleEndian.Uint16(xst[0:2]))
	if cch <= 0 || 2+cch*2 > len(xst) {
		return ""
	}
	placeholders := make(map[int]int, 9) // xst-char-index → 1-based level
	// MS-DOC rgbxchNums offsets are zero-based; 0xFF means that the level is
	// absent from the number text. Older office-renderer output used a 1-based
	// encoding with zero as the sentinel, so accept that form as a compatibility
	// fallback while preferring the native DOC representation whenever 0xFF is
	// present.
	nativeOffsets := false
	for slot := 0; slot < 9; slot++ {
		if lvlfHeader[6+slot] == 0xFF {
			nativeOffsets = true
			break
		}
	}
	for slot := 0; slot < 9; slot++ {
		raw := int(lvlfHeader[6+slot])
		idx := raw
		if nativeOffsets {
			if raw == 0xFF {
				continue
			}
		} else {
			if raw == 0 {
				continue
			}
			idx = raw - 1
		}
		if idx < 0 || idx >= cch {
			continue
		}
		placeholders[idx] = slot + 1
	}
	out := make([]rune, 0, cch+8)
	for i := 0; i < cch; i++ {
		ch := binary.LittleEndian.Uint16(xst[2+i*2 : 2+i*2+2])
		if level, ok := placeholders[i]; ok {
			out = append(out, '%')
			out = append(out, rune('0'+level))
			continue
		}
		out = append(out, rune(ch))
	}
	return string(out)
}

// parseLegacyPlfLfoBlob decodes a simplified PlfLfo-like payload:
// [count:uint32][lsid:uint32]*count with optional trailing per-entry start
// overrides decoded separately by parseLegacyPlfLfoStartOverrides. ilfo values
// are 1-based by entry index.
func parseLegacyPlfLfoBlob(blob []byte) map[int]uint32 {
	if len(blob) < 8 {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(blob[0:4]))
	if count <= 0 || count > 128 || len(blob) < 4+count*4 {
		return nil
	}
	out := make(map[int]uint32, count)
	for i := 0; i < count; i++ {
		lsid := binary.LittleEndian.Uint32(blob[4+i*4 : 8+i*4])
		if lsid == 0 {
			continue
		}
		out[i+1] = lsid
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLegacyPlfLfoStartOverrides decodes optional trailing per-ilfo starts
// from the same simplified PlfLfo-like payload:
// [count:uint32][lsid:uint32]*count[startLevel0..startLevel8]*count.
// In this simplified block, 0xFE explicitly clears an inherited start while
// 0xFF (and 0) leaves the level unset.
func parseLegacyPlfLfoStartOverrides(blob []byte) map[string]int {
	block, ok := legacyPlfLfoOverrideBlock(blob, 0)
	if !ok {
		return nil
	}
	out := make(map[string]int)
	count := len(block) / 9
	for i := 0; i < count; i++ {
		starts := block[i*9 : (i+1)*9]
		for level, start := range starts {
			if start == 0 || start == 0xFF {
				continue
			}
			if start == 0xFE {
				out[legacyListKey(i+1, level)] = legacyListOverrideClear
				continue
			}
			out[legacyListKey(i+1, level)] = int(start)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPlfLfoFormatOverrides(blob []byte) map[string]string {
	block, ok := legacyPlfLfoOverrideBlock(blob, 1)
	if !ok {
		return nil
	}
	out := make(map[string]string)
	count := len(block) / 9
	for i := 0; i < count; i++ {
		formats := block[i*9 : (i+1)*9]
		for level, nfc := range formats {
			format := legacyNfcToListFormat(nfc)
			if format == "" {
				continue
			}
			out[legacyListKey(i+1, level)] = format
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLegacyPlfLfoRestartOverrides decodes optional trailing per-ilfo restart
// hints. In this simplified block, 0x01 enables restart, 0xFE clears an
// inherited restart hint, and 0xFF (and 0) leaves the level unset.
func parseLegacyPlfLfoRestartOverrides(blob []byte) map[string]int {
	block, ok := legacyPlfLfoOverrideBlock(blob, 2)
	if !ok {
		return nil
	}
	out := make(map[string]int)
	count := len(block) / 9
	for i := 0; i < count; i++ {
		restarts := block[i*9 : (i+1)*9]
		for level, restart := range restarts {
			if restart == 0 || restart == 0xFF {
				continue
			}
			if restart == 0xFE {
				out[legacyListKey(i+1, level)] = legacyListOverrideClear
				continue
			}
			out[legacyListKey(i+1, level)] = int(restart)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLegacyPlfLfoRestartOnLevelOverrides decodes optional trailing per-ilfo
// restart targets. In this simplified block, 0xFE clears an inherited target,
// 0xFF leaves the level unset, and values 0..8 pick the triggering parent.
func parseLegacyPlfLfoRestartOnLevelOverrides(blob []byte) map[string]int {
	block, ok := legacyPlfLfoOverrideBlock(blob, 3)
	if !ok {
		return nil
	}
	out := make(map[string]int)
	count := len(block) / 9
	for i := 0; i < count; i++ {
		targets := block[i*9 : (i+1)*9]
		for level, target := range targets {
			if target == 0xFF {
				continue
			}
			if target == 0xFE {
				out[legacyListKey(i+1, level)] = legacyListOverrideClear
				continue
			}
			if target > 8 {
				continue
			}
			out[legacyListKey(i+1, level)] = int(target)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func legacyPlfLfoOverrideBlock(blob []byte, blockIndex int) ([]byte, bool) {
	if len(blob) < 17 {
		return nil, false
	}
	count := int(binary.LittleEndian.Uint32(blob[0:4]))
	if count <= 0 || count > 128 {
		return nil, false
	}
	off := 4 + count*4 + blockIndex*count*9
	end := off + count*9
	if end > len(blob) {
		return nil, false
	}
	return blob[off:end], true
}

// parseLegacyPlfLstBlob decodes a minimal PlfLst/LVLF-like payload:
// [count:uint32][lsid:uint32][nfcLevel0..nfcLevel8]*count.
func parseLegacyPlfLstBlob(blob []byte) map[uint32]legacyListLevelData {
	if structured, ok := parseLegacyStructuredPlfLstBlob(blob); ok {
		return structured
	}
	if len(blob) < 17 {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(blob[0:4]))
	if count <= 0 || count > 128 {
		return nil
	}
	hasStart := false
	hasRestart := false
	hasRestartOnLevel := false
	if len(blob) >= 4+count*22 {
		hasStart = true
	}
	if len(blob) >= 4+count*31 {
		hasRestart = true
	}
	if len(blob) >= 4+count*40 {
		hasRestartOnLevel = true
	} else if len(blob) < 4+count*13 {
		return nil
	}
	out := make(map[uint32]legacyListLevelData, count)
	off := 4
	for i := 0; i < count; i++ {
		lsid := binary.LittleEndian.Uint32(blob[off : off+4])
		off += 4
		levels := append([]byte(nil), blob[off:off+9]...)
		off += 9
		starts := make([]int, 9)
		restarts := make([]bool, 9)
		restartOnLevel := make([]int, 9)
		for j := 0; j < len(restartOnLevel); j++ {
			restartOnLevel[j] = -1
		}
		if hasStart {
			for j := 0; j < 9; j++ {
				starts[j] = int(blob[off+j])
			}
			off += 9
		}
		if hasRestart {
			for j := 0; j < 9; j++ {
				restarts[j] = blob[off+j] != 0
			}
			off += 9
		}
		if hasRestartOnLevel {
			for j := 0; j < 9; j++ {
				v := int(blob[off+j])
				if v >= 0 && v <= 8 {
					restartOnLevel[j] = v
				}
			}
			off += 9
		}
		if lsid == 0 {
			continue
		}
		out[lsid] = legacyListLevelData{format: levels, start: starts, restart: restarts, restartOnLevel: restartOnLevel}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyStructuredPlfLstBlob(blob []byte) (map[uint32]legacyListLevelData, bool) {
	out, _, size, ok := parseLegacyStructuredPlfLstPrefix(blob)
	if !ok || size != len(blob) {
		return nil, false
	}
	return out, true
}

func parseLegacyStructuredPlfLstPrefix(blob []byte) (map[uint32]legacyListLevelData, int, int, bool) {
	if len(blob) < 2+legacyLstfSize {
		return nil, 0, 0, false
	}
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	if count <= 0 || count > 128 {
		return nil, 0, 0, false
	}
	lstfOffset := 2
	lvlOffset := lstfOffset + count*legacyLstfSize
	if lvlOffset > len(blob) {
		return nil, 0, 0, false
	}
	out := make(map[uint32]legacyListLevelData, count)
	off := lvlOffset
	score := 0
	for i := 0; i < count; i++ {
		entryOffset := lstfOffset + i*legacyLstfSize
		lsid := binary.LittleEndian.Uint32(blob[entryOffset : entryOffset+4])
		levelCount := 9
		if blob[entryOffset+26]&legacyLSTFFlagSimpleList != 0 {
			levelCount = 1
		}
		formats := make([]byte, levelCount)
		starts := make([]int, levelCount)
		restarts := make([]bool, levelCount)
		restartOnLevel := make([]int, levelCount)
		indentLeft := make([]int, levelCount)
		indentFirstLine := make([]int, levelCount)
		templates := make([]string, levelCount)
		hasRunFormats := make([]bool, levelCount)
		runFormats := make([]wordir.RunFormat, levelCount)
		for level := range restartOnLevel {
			restartOnLevel[level] = -1
		}
		for level := 0; level < levelCount; level++ {
			lvl, size, ok := parseLegacyStructuredLVL(blob[off:])
			if !ok {
				return nil, 0, 0, false
			}
			off += size
			formats[level] = lvl.format
			starts[level] = lvl.start
			restarts[level] = lvl.hasRestart && lvl.restart
			if lvl.hasRestartOnLevel {
				restartOnLevel[level] = lvl.restartOnLevel
			}
			indentLeft[level] = lvl.indentLeft
			indentFirstLine[level] = lvl.indentFirstLine
			templates[level] = lvl.template
			hasRunFormats[level] = lvl.hasRunFormat
			runFormats[level] = lvl.runFormat
			if legacyNfcToListFormat(lvl.format) != "" {
				score += 10
			}
			if lvl.start > 0 {
				score++
			}
			if lvl.hasRestart && lvl.restart {
				score++
			}
			if lvl.hasRestartOnLevel {
				score++
			}
		}
		if lsid != 0 {
			out[lsid] = legacyListLevelData{format: formats, start: starts, restart: restarts, restartOnLevel: restartOnLevel, indentLeft: indentLeft, indentFirstLine: indentFirstLine, template: templates, runFormat: runFormats, hasRunFormat: hasRunFormats}
		}
	}
	if len(out) == 0 {
		return nil, 0, 0, false
	}
	return out, score, off, true
}

func legacyPlfLstScore(levels map[uint32]legacyListLevelData) int {
	if len(levels) == 0 {
		return 0
	}
	score := 0
	for _, levelData := range levels {
		for level, nfc := range levelData.format {
			if legacyNfcToListFormat(nfc) == "" {
				continue
			}
			score += 10
			if level < len(levelData.start) && levelData.start[level] > 0 {
				score++
			}
			if level < len(levelData.restart) && levelData.restart[level] {
				score++
			}
			if level < len(levelData.restartOnLevel) && levelData.restartOnLevel[level] >= 0 {
				score++
			}
		}
	}
	return score
}

// legacyListFormatForTemplate treats the LVLF number-text template as the
// strongest signal for bullet-vs-numbered semantics. Literal templates contain
// no level placeholder and represent bullet glyphs; some producers retain a
// stale numeric nfc in an LFO formatting override.
func legacyListFormatForTemplate(format, template string) string {
	template = strings.TrimSpace(template)
	if template != "" && !strings.Contains(template, "%") {
		return "bullet"
	}
	return format
}

func legacyNfcToListFormat(nfc byte) string {
	switch nfc {
	case 0:
		return "decimal"
	case 1:
		return "upperRoman"
	case 2:
		return "lowerRoman"
	case 3:
		return "upperLetter"
	case 4:
		return "lowerLetter"
	case 0x17:
		return "bullet"
	default:
		return ""
	}
}
