// OOXML variants use the same Word, Excel, and PowerPoint packages as their
// base document types. The plugin must claim them too so they do not fall back
// to Manager's converter-based document viewer.
const EXTENSIONS = new Set([
  'doc', 'docx', 'docm', 'dotx', 'dotm',
  'xls', 'xlsx', 'xlsm', 'xltx', 'xltm', 'xlam',
  'ppt', 'pptx', 'pptm', 'ppsx', 'ppsm', 'potx', 'potm',
]);

function extension(path) {
  const name = String(path ?? '').replace(/\\/g, '/').split('/').pop() ?? '';
  const dot = name.lastIndexOf('.');
  return dot < 0 ? '' : name.slice(dot + 1).toLowerCase();
}

export async function activate(ctx) {
  if (!ctx?.api?.registerDocumentRenderer || !ctx?.backend?.call) {
    throw new Error('This version of Manager does not support plugin document renderers.');
  }

  ctx.api.registerDocumentRenderer({
    id: 'aether.office-renderer',
    title: 'Office Document Renderer',
    score(request) {
      return EXTENSIONS.has(extension(request?.archiveContext?.entryPath || request?.targetPath)) ? 100 : 0;
    },
    async render(request) {
      const path = request?.targetPath;
      if (!path || request?.archiveContext) {
        throw new Error('Office previews are available for local files only.');
      }
      const result = await ctx.backend.call('office.render', { path });
      if (!result?.html) throw new Error('Office renderer returned no HTML.');
      return { html: String(result.html) };
    },
  });
}
