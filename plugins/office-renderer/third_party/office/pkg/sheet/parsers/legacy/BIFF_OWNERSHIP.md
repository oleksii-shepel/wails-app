# BIFF parser ownership

Production legacy spreadsheet reads have one canonical stream boundary:

`XLS/CFB -> decrypted BIFF stream -> ParseBIFFStream -> canonical Sheet IR`

Workbook-global semantic records are collected by one sequential pass through
`parseWorkbookGlobalsInto`. That pass owns workbook metadata, calculation
settings, external-link SupBooks, pivot-cache sources, styles, shared strings,
defined names, protection and workbook drawing-group state.

`parseLegacyExternalLinks`, `parseLegacyPivotCacheSources` and
`parseLegacyWorkbookMetadata` remain focused decoder/oracle helpers for tests.
They are not production composition points.

`findSheetBOFOffsets` intentionally remains separate. It scans beyond workbook
globals into worksheet substreams and exists only to recover invalid BoundSheet
offsets; it is structural recovery, not a second workbook-global semantic pass.

Worksheet parsing consumes `legacyWorkbookParseContext` and projects directly
into canonical `sheet/ir`.

## Worksheet substreams

Each worksheet is still decoded in one sequential BIFF pass, but all transient
cross-record state is owned by `legacyWorksheetParseState`. This includes
FORMULA/STRING continuations, shared and array formulas, NOTE/OBJ/TXO comment
assembly, control and drawing text, conditional-format headers, protection,
hyperlinks, OfficeArt payloads, pivot views and table-feature assembly.

Flush/finalization operations for record-spanning text, formulas and pivot views
are methods on that state. `parseWorksheetWithDrawingStore` owns orchestration
and canonical `Sheet`/`Row` output; it no longer owns dozens of unrelated loose
state variables.

## Worksheet record families

The sequential worksheet decoder now has an explicit semantic-family boundary:
cell/formula, geometry/view, object/comment, worksheet semantics, pivot/table and
core/control records. The first family moved fully behind that boundary is the
pivot-view record sequence (`SXVIEW`, `SXVD`, `SXVI`, `SXIVD`, `SXPI`, `SXDI`).
The worksheet loop classifies and dispatches it as one unit rather than owning
the individual BIFF cases.

The classifier is intentionally architectural rather than one-handler-per-record:
future extraction moves whole record families while preserving one physical
worksheet pass and the same `legacyWorksheetParseState`.

### Cell and formula family

Cell/value and formula records are now fully outside the worksheet orchestrator.
The family handler owns LABEL/SST, blanks, NUMBER/RK/MULRK, BOOLERR, rich
strings, FORMULA-to-STRING continuation, array formulas, shared formulas and
numeric date canonicalization. When a STRING consumes BIFF CONTINUE records, the
handler reports the advanced stream position explicitly so the outer loop
remains the sole physical stream cursor.

### Geometry and view family

Worksheet physical geometry and view state now live behind one family handler.
It owns WINDOW2 freeze state, PANE, DIMENSIONS, GUTS/outlines, SCL zoom, ROW,
COLINFO, merged ranges, default row/column sizing and pane selections. Custom
view aggregates are consumed as a unit so their page settings cannot leak into
the active worksheet.

Merged-cell and custom-view aggregates report their advanced stream position to
the outer loop. The worksheet orchestrator therefore remains the sole physical
BIFF cursor owner while semantic aggregate consumption stays inside the family.

### Object, comment and drawing family

NOTE, OBJ, MSODRAWING, TXO and their CONTINUE records now live behind one
object-family handler. The handler owns comment association and rich text,
form-control construction and linked formulas, drawing text, OfficeArt payload
aggregation, and the record-spanning TXO continuation protocol.

The outer worksheet loop still owns the physical cursor. MSODRAWING aggregates
and consumed TXO CONTINUE records return their advanced position explicitly.
This keeps object parsing sequential without creating a second drawing/comment
parser over the worksheet stream.

### Worksheet semantic family

Interactive and page-level worksheet semantics now live behind one family
handler. It owns hyperlinks, conditional-format groups and rules, data
validations, auto filters, worksheet/object/scenario protection, password hash,
page setup, margins, centering/print flags, headers and footers, and row/column
page breaks.

This family does not own the physical BIFF cursor: its records are single-record
semantic projections. Cross-record state such as conditional-format group
headers and deferred filter/protection projection remains in
legacyWorksheetParseState and is finalized with the worksheet.

### Table and shared-feature family

Legacy table projection now has explicit worksheet-family ownership. FEAT,
FEATURE11/12 and LIST12 records construct canonical tables, table columns,
styles and display names behind one handler. Future-record CONTINUE aggregates
report their advanced position to the outer sequential cursor.

With this extraction, the worksheet switch is no longer the semantic parser for
cells, geometry, objects, pivots, worksheet features, or tables. It is reduced
to substream control plus intentionally ignored BIFF bookkeeping records.

### Canonical worksheet finalization

Record consumption and canonical assembly are separate ownership phases.
`parseWorksheetWithDrawingStore` owns the sequential BIFF cursor and dispatch;
`legacyWorksheetParseState.finalizeWorksheet` owns all deferred projection after
the worksheet substream ends.

The finalizer resolves pending text, formula metadata, comments, drawings and
shapes, form controls, pivot views, auto-filter ranges, protection, hyperlinks,
row outlines, and empty-map normalization. This prevents post-loop semantic
assembly from accumulating again in the physical record orchestrator.

### Canonical worksheet dispatcher

The production worksheet entrypoint is now `parseBIFFWorksheet`. Its former
`parseWorksheetWithDrawingStore` name described an implementation detail rather
than the parser boundary and is retired.

EOF/BOF handling, embedded chart-substream skipping, and opaque/index
bookkeeping records are isolated behind `consumeWorksheetControlRecord`.
Consequently the canonical worksheet parser contains no inline semantic or
control-record switch: it owns the physical cursor, dispatches record families,
then delegates canonical assembly to `finalizeWorksheet`.

### XLS/XLSX canonical parity closure

The BIFF frontend now projects three canonical worksheet semantics that were
previously weaker than the XLSX frontend:

- WINDOW2 explicitly owns `Sheet.ShowGridLines`.
- the built-in `_xlnm._FilterDatabase` defined name is the authoritative
  `Sheet.AutoFilterRef` when present, replacing the AUTOFILTERINFO-derived
  compatibility estimate.
- SHRFMLA groups are preserved in `Sheet.SharedFormulas`, not only expanded
  into per-cell formulas.

These are canonical projections, not renderer-specific compatibility hacks.

### XLS/XLSX canonical parity: print and table metadata

The BIFF frontend now preserves extended HEADERFOOTER option flags in the same
canonical fields used by XLSX: different odd/even pages, different first page,
scale-with-document, and align-with-margins.

Legacy structured tables now also project their embedded `AutoFilterRef`.
The range follows the table body/header extent and excludes authored totals
rows, matching the canonical table model rather than relying only on the
worksheet-level AutoFilter compatibility view.

### XLS/XLSX canonical parity: hyperlinks and table ranges

The BIFF worksheet semantic family now recognizes `HLINKTOOLTIP` and joins its
screen-tip metadata to the canonical `Hyperlink.Tooltip` projection produced
from `HLINK`.

Legacy table range parsing also accepts canonical absolute A1 references and
uses the full table bounds when deriving the embedded AutoFilter range. Totals
rows are excluded from that range, including multiple totals rows.

### XLS/XLSX canonical parity: pivot axes and table filter ownership

BIFF `SXVD` data-axis membership is now retained as canonical
`PivotField.DataField`, instead of being represented only indirectly by
`PivotTable.DataFields`.

Structured-table AutoFilter projection now also respects header ownership:
headerless legacy tables do not synthesize an embedded AutoFilter range.
Tables with headers retain the canonical filter range, excluding authored
totals rows.

### Canonical XLS completion gate

`XLS_CANONICAL_PARITY.md` defines the corpus-driven completion contract.
Once record-family ownership is established, unused BIFF constants are not
treated as a backlog by themselves. Canonical IR coverage and corpus behavior
decide whether a remaining record is a parity gap.

### XLS final closure

The completion contract is deliberately end-to-end: dynamic corpus discovery,
canonical invariants, XLS-to-XLSX semantic round-trip parity, and deterministic
HTML rendering. This prevents a parser-only green state from hiding losses in
conversion or rendering.

### Final production cleanup

The raw-BIFF fallback now enters `parseBIFFWorksheet` directly. The historical
`parseWorksheet` wrapper is test-only and cannot become a production ownership
boundary again. `XLS_COMPLETION.md` is the explicit supported/unsupported
completion inventory.
