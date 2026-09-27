# Metafile playback coverage

This package is the browser-independent playback boundary for Office WMF, EMF, and EMF+ artwork.

## Closure policy

A record family is considered **closed** when playback is implemented for the semantics that materially affect SVG output and has focused tests. Device/printer negotiation, OpenGL passthrough, ICM/profile management, and reserved/multiformat transport records are intentionally **non-rendering** and are documented as such rather than emulated.

## WMF

### Closed rendering/state families

- Mapping/state: SaveDC/RestoreDC, map mode, window/viewport origin+extent, window/viewport scaling, background color/mode, polygon fill mode, text color/alignment/character extra.
- Objects: create/select/delete pen, brush, font.
- Vector drawing: MoveTo/LineTo, Polygon, Polyline, PolyPolygon, Rectangle, RoundRect, Ellipse, Arc, Chord, Pie, SetPixel.
- Text: TextOut, ExtTextOut.
- Raster: SetDIBToDev, StretchDIB for packed DIBs handled by the shared DIB decoder.
- WMFC `META_ESCAPE_ENHANCED_METAFILE`: reconstructs and dispatches the embedded EMF stream.

### Explicit non-rendering / partial WMF families

- Printer-driver and document-control escapes other than WMFC embedded EMF.
- FloodFill/ExtFloodFill and region inversion/frame operations where exact destination-surface readback would be required.
- Pattern ROP bitmap operations that require destination readback or device palette behavior.
- Palette-management records and device-specific color management.

## EMF

### Closed rendering/state families

- Window/viewport/map/world transforms, SaveDC/RestoreDC, layout, mapper flags, arc direction, polygon fill, brush origin, text state.
- Pens/extended pens, brushes, DIB/mono pattern brushes, fonts and stock-object selection.
- Lines, polygons, polypolygons/polylines, Beziers, paths, rectangles, ellipses, roundrects, Arc/ArcTo/AngleArc/Chord/Pie, SetPixelV.
- Region fill/paint and rectangle/region/path clipping, including canonical rectangle-region boolean composition.
- Unicode text: ExtTextOutW, PolyTextOutW, SmallTextOut; authored Dx/PDY, opaque/clipped text, font escapement/orientation, underline/strikeout and layout state.
- Raster: SetDIBitsToDevice, StretchDIBits, BitBlt, StretchBlt, MaskBlt subset with representable source-copy semantics, PlgBlt, bitfield/alpha DIBs.
- GradientFill rectangles/triangles.
- EMF+ GDI comments are isolated and delegated to the EMF+ engine.

### Explicit non-rendering / partial EMF families

- OpenGL records, printer escapes, color-space/ICM/profile records, palette-management records, device pixel-format records.
- Destination-dependent raster operations that cannot be represented without framebuffer readback.
- AlphaBlend and TransparentBlt are handled by the shared raster compositor; destination-readback ROPs remain explicitly partial.
- ExtFloodFill and region inversion/frame semantics that depend on destination-surface readback.
- ANSI ExtTextOutA/PolyTextOutA are not promoted; Office Unicode output is handled by the W variants.

## EMF+

### Closed rendering/state families

- All EMF+ drawing record types: Clear, Fill/Draw Rects, Polygon/Lines, Ellipse, Pie, Arc, Region, Path, ClosedCurve, Curve, Beziers, Image/ImagePoints, String and DriverString.
- Object table/continuations: brushes, pens, paths, regions, images, fonts, StringFormat and ImageAttributes.
- Relative PointR coordinate decoding and RLE PathPointTypes for compact EMF+ paths/drawing records.
- Solid, hatch/pattern, linear-gradient and texture brush playback used by Office artwork.
- Standard/advanced PenData state: start/end caps, joins, miter limit, line styles, custom dash arrays/offsets, non-center alignment metadata, standard cap markers, compound-line arrays and nested custom start/end-cap payload parsing. Adjustable-arrow custom caps and default custom-cap paths are materialized as SVG markers; compound cross-section arrays are preserved exactly as authored metadata for a stroke-outline backend.
- Region tree parsing and exact mask algebra for Replace/Intersect/Union/Xor/Exclude/Complement clipping.
- Save/Restore, containers, world/page transforms, clipping, text-rendering state, compositing state preservation, rendering origin, anti-alias, interpolation and pixel-offset quality state.
- Bitmap image objects, source cropping and destination parallelograms; ImageAttributes wrap/clamp semantics.
- GDI+ 1.1 SerializableObject image effects: BlurEffect, ColorMatrixEffect, Brightness/Contrast, ColorBalance, Hue/Saturation/Lightness, Levels, Sharpen and Tint.
- Font realization through supplied SFNT faces, cmap/hmtx/kern/GPOS pair positioning, DrawString layout and DrawDriverString authored positions.

### Explicit partial EMF+ families

- Compound-line arrays are parsed and preserved, but exact arbitrary cross-section playback requires stroke-to-outline/offset-curve geometry that SVG 1.1 does not provide natively.
- SerializableObject ColorCurve, ColorLookupTable and RedEyeCorrection remain backend-limited: exact playback requires per-channel lookup tables or region-aware retained-pixel processing.
- EmfPlusStrokeFillPath (0x4037) is enumerated by MS-EMFPLUS but no normative wire payload is defined in the current specification; playback deliberately does not guess a record layout.
- Exact SourceCopy destination replacement and other destination-dependent raster operations are retained as semantic metadata; a stateless SVG emitter cannot read or replace already-painted destination pixels.
- Full complex-script shaping/GSUB/mark/cursive positioning is outside metafile playback and belongs to the shared font/shaping subsystem.
- Terminal-server and reserved multiformat transport records are non-rendering.

## Regression corpus

`37625.pptx` is the primary mixed-metafile regression corpus. Its embedded WMF/EMF assets are expected to remain convertible after every playback change.

## Final closure boundary

The parser/playback layer is considered closed when every rendering record is either:

1. decoded and materialized to SVG/vector/raster semantics; or
2. decoded/preserved with an explicit backend requirement where exact output fundamentally needs capabilities unavailable to a stateless SVG emitter (destination framebuffer readback, arbitrary stroke-to-outline offset geometry, or region-aware pixel effects).

At this boundary, remaining differences are backend capability gaps rather than unknown WMF/EMF/EMF+ wire-format gaps.
