# Legacy PPT parser completeness — Pass 4

Pass 4 starts consuming the ownership ledger as a repair driver.

## Closed ownership gap

Picture-frame `SpContainer`s were globally excluded from generic OfficeArt projection. The picture family only materializes a shape when it has usable placement plus a BLIP or external-object reference. Consequently an authored picture-frame container with intact OfficeArt geometry/paint/anchor but a missing or unusual media relationship became `unclaimed` and disappeared.

Ownership is now outcome-based: a successful picture/media/OLE projection claims the ShapeID first; otherwise the generic OfficeArt projection is allowed to preserve the authored frame. This keeps the complete OfficeArt visual and source identity without duplicating successfully resolved pictures.

## Completeness boundary repair

Pass 3's `completeProjection` implementation returned `legacyPptVisualProjection` while its declared return type was still the old shape slice. Pass 4 corrects the declaration so production callers and the ownership tests agree on the projection contract.

## Regression coverage

`TestParserCompletenessUnresolvedPictureFrameFallsBackToCanonicalOfficeArt` proves that an unresolved picture-frame container with valid authored placement reaches canonical IR and leaves no unclaimed ownership entry.

## Next

Use remaining `unclaimed` entries to close placement sparsity: distinguish malformed/missing anchors from unresolved group-coordinate ownership, then recover containers for which placement exists in another authoritative OfficeArt representation.
