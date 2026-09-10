# XLS completion contract

The production XLS frontend is considered complete for the supported Sheet
model when all of the following remain green:

1. every `.xls` fixture in `testdata/xls` enters the canonical corpus gate;
2. XLS -> canonical IR -> XLSX -> canonical IR preserves supported semantics;
3. the XLS corpus renders without unexpected diagnostics;
4. repeated rendering is deterministic;
5. the repeated legacy parser gate remains stable.

## Canonical coverage

The BIFF frontend projects every currently supported canonical `Sheet` field
through the legacy parser surface. The workbook-level inventory has three
fields without BIFF document-semantic projection:

- `Connections`
- `EmbeddedFonts`
- `Diagnostics`

These are not silently claimed as supported XLS semantics.

`Diagnostics` is parser/runtime infrastructure rather than document content.
`Connections` and `EmbeddedFonts` remain explicit unsupported XLS capabilities
until a supported BIFF corpus fixture and a defined canonical mapping justify
implementation. Their absence must not be hidden by synthetic defaults.

## Unsupported-feature policy

An unused BIFF record constant is not itself a parity bug. A record is a
completion blocker only when:

- it carries document semantics represented by canonical Sheet IR,
- that semantic surface is part of the supported Excel product contract, and
- the BIFF frontend fails to preserve it.

Records with no canonical target remain explicitly outside the supported
surface rather than being projected into ad-hoc legacy-only state.

Known record families that must not be treated as an automatic TODO list
include COUNTRY, HIDEOBJ, GRIDSET, drawing-selection bookkeeping, interface
records, and other workbook bookkeeping records.

Pivot extensions such as SXEX/SXEXTDF and legacy query/connection records are
to be implemented only from an actual semantic contract/specification and
corpus evidence; fixture-specific decoding is not acceptable.

## Production ownership

Production paths converge on:

    CFB/raw BIFF
        -> workbook stream/decryption
        -> ParseBIFFStream
        -> workbook-global parse context
        -> parseBIFFWorksheet
        -> canonical Sheet IR

The historical `parseWorksheet` wrapper and focused
`parseWorkbookGlobals` return surface remain test/oracle helpers only.
The raw-BIFF production fallback now calls `parseBIFFWorksheet` directly so
there is no second worksheet ownership path.

## Closure rule

A new XLS fixture that exposes loss in canonical IR, XLSX conversion, or
rendering reopens XLS parity. Snapshot regeneration alone is never proof of
correctness. Unsupported features must be listed here rather than silently
ignored or approximated.
