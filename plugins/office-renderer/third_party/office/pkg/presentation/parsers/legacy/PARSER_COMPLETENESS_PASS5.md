# Legacy PPT parser completeness — Pass 5

## Placement and coordinate-space takeover

Pass 5 makes OfficeArt placement a single parser policy instead of repeating anchor precedence independently in text, picture, connector and generic-shape families.

`resolveLegacyPptShapePlacement` now owns the rules:

- child shapes prefer `ChildAnchor` mapped through the active group coordinate frame;
- a valid `ClientAnchor` is the authoritative slide-space fallback when group placement cannot be resolved;
- raw `ChildAnchor` coordinates are accepted directly only for non-child shapes;
- unresolved child group coordinates are never leaked into slide-space canonical bounds;
- failure carries a concrete reason (`no authored anchor`, unresolved group frame, malformed/unusable anchor).

Text, picture, connector and generic OfficeArt projection all consume this policy. The per-`SpContainer` ownership audit also uses it, so an unclaimed container now distinguishes a placement gap from a semantic-family ownership gap.

This closes the architectural class where family-specific anchor selection could disagree and gives the corpus takeover a useful classification for the remaining sparse-placement cases.
