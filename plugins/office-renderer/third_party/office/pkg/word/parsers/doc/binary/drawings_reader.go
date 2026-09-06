package docbinary

import "io"

// ReadFibRangeAt reads one FIB-addressed table range without materializing the
// whole table stream. minSize is the smallest useful record size for the caller.
func ReadFibRangeAt(table io.ReaderAt, tableSize int64, fib []byte, fibBase, cbRgFcLcb, index, minSize int) ([]byte, bool) {
	if table == nil || tableSize < 0 || index < 0 || index >= cbRgFcLcb || fibBase < 0 {
		return nil, false
	}
	fc, lcb, ok := FibRange(fib, fibBase, cbRgFcLcb, index)
	if !ok || lcb < minSize || int64(fc) < 0 || int64(fc)+int64(lcb) > tableSize {
		return nil, false
	}
	blob := make([]byte, lcb)
	if _, err := table.ReadAt(blob, int64(fc)); err != nil && err != io.EOF {
		return nil, false
	}
	return blob, true
}

// FloatingAnchorsReaderAt loads the authoritative main-story PlcfSpa and keeps
// the historical malformed-producer rescue in the canonical binary layer.
// Alternative FIB slots are considered only when the authoritative slot cannot
// produce a valid anchor PLC.
func FloatingAnchorsReaderAt(table io.ReaderAt, tableSize int64, fib []byte, fibBase, cbRgFcLcb, mainTextLen int) []FloatingAnchor {
	if table == nil || mainTextLen < 0 {
		return nil
	}
	if blob, ok := ReadFibRangeAt(table, tableSize, fib, fibBase, cbRgFcLcb, FibPlcSpaMom, 34); ok {
		if anchors, valid := ParsePlcfSpa(blob, mainTextLen); valid && len(anchors) > 0 {
			return anchors
		}
	}

	var best []FloatingAnchor
	bestScore := -1
	for index := 0; index < cbRgFcLcb; index++ {
		if index == FibPlcSpaMom {
			continue
		}
		blob, ok := ReadFibRangeAt(table, tableSize, fib, fibBase, cbRgFcLcb, index, 34)
		if !ok {
			continue
		}
		anchors, valid := ParsePlcfSpa(blob, mainTextLen)
		if !valid || len(anchors) == 0 {
			continue
		}
		score := floatingAnchorRecoveryScore(anchors)
		if score > bestScore {
			bestScore = score
			best = anchors
		}
	}
	return best
}

func floatingAnchorRecoveryScore(anchors []FloatingAnchor) int {
	score := 0
	for _, anchor := range anchors {
		if anchor.Shape != 0 {
			score++
		}
		if anchor.Right > anchor.Left && anchor.Bottom > anchor.Top {
			score += 2
		}
		if anchor.Left >= 0 && anchor.Top >= 0 {
			score++
		}
	}
	return score
}
