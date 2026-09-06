package legacy

import (
	"sort"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

const (
	legacyFibPlcftxbxTxt    = docbinary.FibPlcftxbxTxt
	legacyFibPlcfHdrtxbxTxt = docbinary.FibPlcfHdrtxbxTxt
	legacyFTXBXSSize        = 22
	legacyFTXBXSLidOffset   = 14
)

func legacyTextboxAnchorByShapeID(wordStream, tableStream []byte, fibBase, cbRgFcLcb int, counts legacyStoryCounts, mainAnchors map[int]legacyFloatingDrawingAnchor) map[uint32]legacyFloatingDrawingAnchor {
	anchors:=make([]docbinary.FloatingAnchor,0,len(mainAnchors)+4);for _,anchor:=range mainAnchors{anchors=append(anchors,anchor)}
	hdrBase:=counts.Text+counts.Ftn
	anchors=append(anchors,docbinary.FloatingAnchorsAt(wordStream,tableStream,fibBase,cbRgFcLcb,docbinary.FibPlcSpaHdr,hdrBase,counts.Hdr,true)...)
	return docbinary.AnchorMapByShapeID(anchors)
}
func canonicalLegacyTextFlows(streams ...[]byte) map[uint32]string { return docofficeart.DecodeStreams(streams...).TextFlows() }

func legacyCanonicalStoryCounts(counts legacyStoryCounts) docbinary.StoryCounts { return docbinary.StoryCounts{Text:counts.Text,Ftn:counts.Ftn,Hdr:counts.Hdr,Mcr:counts.Mcr,Atn:counts.Atn,Edn:counts.Edn,Txbx:counts.Txbx,HdrTxbx:counts.HdrTxbx} }

// Ownership-free/synthetic documents have no textbox PLC ownership. Materialize
// each textbox story canonically, then split the canonical IR at the page-break
// events produced from legacy form-feed delimiters. No legacy CLX run/story
// builder remains behind this fallback.
func extractLegacyDrawingTextBoxes(wordStream, clxData []byte, counts legacyStoryCounts, chpxEntries []chpxEntry, papxEntries []papxEntry, anchors map[int]legacyFloatingDrawingAnchor, additionalOfficeArtStreams ...[]byte) []*wordir.TextBox {
	if counts.Txbx<=0&&counts.HdrTxbx<=0{return nil}
	opened:=canonicalLosslessStoryCore(clxData,chpxEntries,papxEntries);if opened==nil{return nil}
	storyStart:=docbinary.TextboxStoryBase(legacyCanonicalStoryCounts(counts),false);storyLengths:=[]int{counts.Txbx,counts.HdrTxbx};result:=make([]*wordir.TextBox,0,2)
	for _,storyLen:=range storyLengths{
		if storyLen<=0{continue};storyEnd:=storyStart+storyLen
		story:=materializeCanonicalLosslessStory(wordStream,opened,storyStart,storyEnd);storyStart=storyEnd
		for _,group:=range splitCanonicalStoryOnBreak(story,"page"){
			if len(group.Blocks)==0{continue}
			result=append(result,&wordir.TextBox{Blocks:group.Blocks,Floating:true,WrapType:"square",XRelative:"page",YRelative:"page"})
		}
	}
	if len(result)==0{return nil};textFlows:=canonicalLegacyTextFlows(append([][]byte{wordStream},additionalOfficeArtStreams...)...);sortedAnchors:=legacySortedFloatingDrawingAnchors(anchors);for i:=range result{if i>=len(sortedAnchors){break};applyLegacyAnchorToTextBox(result[i],sortedAnchors[i],textFlows)};return result
}

func legacySortedFloatingDrawingAnchors(anchors map[int]legacyFloatingDrawingAnchor) []legacyFloatingDrawingAnchor { if len(anchors)==0{return nil};cps:=make([]int,0,len(anchors));for cp:=range anchors{cps=append(cps,cp)};sort.Ints(cps);out:=make([]legacyFloatingDrawingAnchor,0,len(cps));for _,cp:=range cps{out=append(out,anchors[cp])};return out }
func applyLegacyAnchorToTextBox(tb *wordir.TextBox, anchor legacyFloatingDrawingAnchor, textFlows map[uint32]string) { if tb==nil||!docbinary.ApplyAnchorToTextBox(tb,anchor){return};if vert,ok:=textFlows[anchor.Shape];ok{tb.Vert=vert;tb.HasVertSetting=true} }

func appendLegacyDetachedDrawingTextBoxes(doc *wordir.Document, textBoxes []*wordir.TextBox) {
	if doc==nil||len(textBoxes)==0{return};remaining:=textBoxes
	for pi:=range doc.Paragraphs{for ri:=range doc.Paragraphs[pi].Runs{run:=&doc.Paragraphs[pi].Runs[ri];if len(remaining)==0{return};if run.Kind=="shape"&&run.Shape!=nil{remaining=docofficeart.AttachTextBoxes(run.Shape,remaining)}}}
	for _,textBox:=range remaining{if textBox==nil||len(textBox.Blocks)==0||textBox.LegacyShapeID!=0{continue};paragraph:=wordir.Paragraph{Index:len(doc.Paragraphs),Runs:[]wordir.Run{{Kind:"textbox",TextBox:textBox}}};doc.Paragraphs=append(doc.Paragraphs,paragraph);doc.Blocks=append(doc.Blocks,wordir.Block{Kind:"paragraph",Paragraph:&doc.Paragraphs[len(doc.Paragraphs)-1]})}
}
