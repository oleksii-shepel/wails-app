import { readFileSync } from 'node:fs';

describe('Office Renderer routing', () => {
  const officeViewer = readFileSync(new URL('./builtins/office-viewer.component.ts', import.meta.url), 'utf8');
  const registry = readFileSync(new URL('./viewer-registry.service.ts', import.meta.url), 'utf8');

  it('keeps plugin-supported Office formats out of LibreOffice conversion', () => {
    expect(officeViewer).toContain('const PLUGIN_RENDERED_EXTENSIONS = new Set([');
    expect(officeViewer).toContain("if (PLUGIN_RENDERED_EXTENSIONS.has(this.fileExt))");
    expect(officeViewer).toContain("this.kind = 'plugin-required';");
    expect(officeViewer.indexOf('if (PLUGIN_RENDERED_EXTENSIONS.has(this.fileExt))')).toBeLessThan(
      officeViewer.indexOf('this.convertDocument();'),
    );
  });

  it('uses the plugin as the only Office document route and keeps PDF direct', () => {
    expect(registry).toContain("id: 'plugin-document'");
    expect(registry).toContain('score: req => this.documentRenderers.resolve(req) ? 90 : 0');
    expect(registry).not.toContain("id: 'office'");
    expect(registry).toContain("id: 'pdf'");
    expect(registry).toContain("extLower(previewPath(req)) === 'pdf' ? 65 : 0");
  });
});
