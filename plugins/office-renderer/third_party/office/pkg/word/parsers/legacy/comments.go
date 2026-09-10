package legacy

func injectDocCommentRefs(runs []binaryWordRun, refByCP map[int]int) []binaryWordRun {
	if len(refByCP) == 0 {
		return runs
	}
	out := make([]binaryWordRun, 0, len(runs)+len(refByCP))
	for _, r := range runs {
		if r.Kind != "" || r.Text == "" {
			out = append(out, r)
			continue
		}
		runes := []rune(r.Text)
		if len(runes) == 0 {
			out = append(out, r)
			continue
		}
		segmentStart := 0
		segmentStartCP := r.StartCP
		cp := r.StartCP
		handled := false
		for i := range runes {
			commentID, ok := refByCP[cp]
			if ok {
				// PLCFANDREF CPs are insertion anchors in the main story; the
				// annotation reference is not a story character. Preserve the rune
				// at the anchor CP and insert the synthetic reference before it.
				handled = true
				if i > segmentStart {
					out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:i]), segmentStartCP))
				}
				out = append(out, binaryWordRun{Kind: "commentRef", CommentID: commentID, Format: r.Format, ParaFormat: r.ParaFormat, StartCP: cp, Revision: r.Revision})
				segmentStart = i
				segmentStartCP = cp
			}
			cp++
		}
		if !handled {
			out = append(out, r)
			continue
		}
		if segmentStart < len(runes) {
			out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:]), segmentStartCP))
		}
	}
	return out
}
