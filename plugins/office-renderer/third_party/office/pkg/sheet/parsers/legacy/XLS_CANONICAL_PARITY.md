# XLS canonical parity gate

The legacy XLS frontend is considered canonically complete when the production
BIFF parser projects supported workbook semantics into the same Sheet IR used
by XLSX and the XLS corpus gate remains green.

## Completion gate

Run:

    go test ./pkg/sheet/... -count=1
    go test ./pkg/sheet/parsers/legacy -count=10

`TestPhase5XLSCanonicalCorpusGate` is the corpus-level semantic gate. It discovers every `.xls` fixture in `testdata/xls` dynamically, so adding a
new corpus workbook automatically expands the completion gate instead of
requiring a hand-maintained fixture list.

The gate checks that every fixture produces a canonical workbook and that
canonical collections retain their invariants. Feature-bearing fixtures also
assert the semantic family they are intended to exercise, including drawings
and external links.

## Remaining completion work

The next closure stage is corpus-driven. A failure is classified as one of:

1. **Projection bug** — BIFF contains semantics represented by Sheet IR, but
   the legacy frontend loses or changes them. This blocks completion.
2. **Unsupported represented feature** — Sheet IR has a canonical target and
   the XLS corpus exercises it, but BIFF support is missing. This blocks
   completion until implemented or deliberately removed from the supported
   surface.
3. **Format-only difference** — XLS and XLSX encode the same semantic result
   differently while canonical IR is equivalent. This does not block.
4. **No canonical IR target** — the BIFF record has no supported semantic
   representation in Sheet IR. Do not add parser ownership merely because a
   BIFF constant exists.

Known workbook-level IR surfaces not currently projected from BIFF are
`Connections` and `EmbeddedFonts`. They become completion blockers only when
the supported XLS corpus/product contract requires those semantics. Diagnostics
are parser infrastructure and are not document-semantic parity.

Unused BIFF constants are not a completion checklist. The corpus plus canonical
IR contract is authoritative.

## Final closure layers

Excel completion is now guarded at three independent levels:

1. **Parse/canonical gate** — every XLS corpus fixture must parse into structurally
   valid canonical Sheet IR.
2. **Semantic round-trip gate** — XLS -> canonical IR -> XLSX -> canonical IR
   must preserve cells and supported workbook/sheet semantics. The parity gate
   compares actual hyperlink, table, pivot, filter, pane, print and
   header/footer semantics rather than feature counts alone.
3. **Render gate** — the XLS corpus must render without unexpected diagnostics,
   and repeated rendering of the same canonical workbook must be byte
   deterministic.

A green result across these layers is the completion criterion for the
supported XLS surface. New fixtures extend the gate automatically.

## Corpus findings closed by the first final gate

The strengthened round-trip gate exposed two cross-layer losses:

- OOXML table part numbering was worksheet-local, allowing later worksheets to
  overwrite earlier `xl/tables/tableN.xml` parts. Table part numbering is now
  workbook-global.
- BIFF pivot semantic field names could differ from source header text. Pivot
  conversion now preserves canonical `PivotTable.Fields` names by positional
  ownership so row/data axes survive XLS -> XLSX conversion.

Malformed legacy NUL-only pivot names are normalized before OOXML serialization
instead of being converted to the XML replacement character.
