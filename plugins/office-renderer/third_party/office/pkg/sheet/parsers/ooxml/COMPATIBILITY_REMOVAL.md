# OOXML parser ownership

The production compatibility bridge is removed.

Production XLSX reads are split into explicit ownership layers:

1. `parts.LoadMetadataPackage` — workbook metadata, styles/theme, shared strings, external links/connections, fonts, and worksheet identity skeletons.
2. `parts.LoadRelationshipObjects` — drawings/images/charts, tables/pivots, VML/form controls, OLE, and header/footer relationship objects.
3. SAX — worksheet rows/cells/formulas, geometry, views, hyperlinks, print/protection, validations, conditional formatting, filters, and comments.
4. `resolve.ResolveMetadataPackage` — projects package metadata/object parts into canonical IR before SAX fills worksheet-owned semantics.

`parts.LoadPackage` and `resolve.ResolvePackage` remain test/tooling oracles only and are forbidden on production read paths.
