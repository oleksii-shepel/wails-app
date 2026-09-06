package word97

import wordir "office/pkg/word/ir"

func cloneIntPtr(v *int)*int{if v==nil{return nil};x:=*v;return &x}

func paragraphSourceFromRun(run Run)*wordir.ParagraphSource{
	return &wordir.ParagraphSource{
		StartCP:run.StartCP,
		PapxSprms:append([]byte(nil),run.PapxSprms...),
		StyleIndex:cloneIntPtr(run.ParagraphFormat.BinaryStyleIndex),
		TableDepth:run.TableDepth,
		LegacyInTableHint:run.LegacyInTableHint,
		LegacyRowMarkHint:run.LegacyRowMarkHint,
		PropertyRevision:sourceRevision(run.PropRMarkIbst,run.HasPropRMarkIbst,run.PropRMarkDttm,run.HasPropRMarkDttm,run.HasPropRMark),
	}
}

func runSourceFromRun(run Run)*wordir.RunSource{
	return &wordir.RunSource{
		ChpxSprms:append([]byte(nil),run.ChpxSprms...),
		StyleIndex:cloneIntPtr(run.Format.BinaryStyleIndex),
		Revision:run.Revision,
		Spec:run.Spec,
		IbstRMark:run.IbstRMark,HasIbstRMark:run.HasIbstRMark,
		DttmRMark:run.DttmRMark,HasDttmRMark:run.HasDttmRMark,
		PropertyRevision:sourceRevision(run.RunPropRMarkIbst,run.HasRunPropRMarkIbst,run.RunPropRMarkDttm,run.HasRunPropRMarkDttm,run.HasRunPropRMark),
		RunPropertyRevision:sourceRevision(run.RunPropRMarkIbst,run.HasRunPropRMarkIbst,run.RunPropRMarkDttm,run.HasRunPropRMarkDttm,run.HasRunPropRMark),
		DisplayFieldRevision:sourceRevision(run.DispFldRMarkIbst,run.HasDispFldRMarkIbst,run.DispFldRMarkDttm,run.HasDispFldRMarkDttm,run.HasDispFldRMark),
	}
}

func sourceRevision(ibst uint16,hasIbst bool,dttm uint32,hasDttm,present bool)*wordir.SourcePropertyRevision{
	if !present&&!hasIbst&&!hasDttm{return nil}
	return &wordir.SourcePropertyRevision{Ibst:ibst,HasIbst:hasIbst,Dttm:dttm,HasDttm:hasDttm}
}
