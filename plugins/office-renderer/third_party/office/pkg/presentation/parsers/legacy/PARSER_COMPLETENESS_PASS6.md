# Legacy PPT parser completeness — Pass 6

Pass 6 makes semantic-family precedence explicit at the OfficeArt scene boundary.

## Ownership rule

A ShapeID is offered to the richest semantic family first: text/table, picture/media/OLE, connector, then generic OfficeArt fallback. A family owns the container only when it actually materializes canonical IR. Generic projection is therefore recovery, not competition.

Connectors need already-materialized shapes to resolve connector-rule sites. The parser now builds a provisional generic endpoint index, projects connectors against that index, discards the provisional projection, and finally commits generic shapes after connector ownership is known. This preserves endpoint binding while preventing a connector SpContainer from being emitted once as a generic AutoShape and again as a connector.

## Completeness invariant

For a valid positioned leaf SpContainer, a specialized decoder declining the container must not by itself make the authored visual disappear. Either a richer family owns the ShapeID or the generic OfficeArt bridge retains it. Structural roots, backgrounds and invalid/missing placement remain explicitly classified by the ownership ledger.

## Regression coverage

`TestParserCompletenessConnectorFamilyClaimsBeforeGenericFallback` proves connector family precedence and single ownership. `TestParserCompletenessGenericFallbackStillOwnsOrdinaryAutoShape` proves the final generic recovery stage remains active for ordinary authored OfficeArt.
