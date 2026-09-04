# Office Document Renderer

Universal renderer plugin for `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, and `.pptx`.
It vendors the Office parser module in `third_party/office`; the Manager itself
has no Office dependency and the plugin does not rely on a sibling checkout.

Build the Windows backend from this directory with:

```powershell
npm run build:backend
```

The plugin registers a declarative document renderer. Manager owns the sandboxed
viewer surface; the plugin backend produces the document HTML.
