import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const archive = readFileSync(new URL('../services/archive.service.ts', import.meta.url), 'utf8');
const sources = readFileSync(new URL('../services/panel-source-registry.service.ts', import.meta.url), 'utf8');
const binding = readFileSync(new URL('../../../../internal/app/bindings_archive_create.go', import.meta.url), 'utf8');

describe('two-way archive pane transfer', () => {
  it('makes top-level ZIP panes writable but keeps nested/non-ZIP archives read-only', () => {
    expect(sources).toContain('create: writableZip');
    expect(sources).toContain('/\\.zip(?:\\/|$)/i.test(context.path)');
  });

  it('routes filesystem-to-archive F5/F6 through archive import', () => {
    expect(dual).toContain('this.archiveService.importPaths(frame, selected)');
    expect(dual).toContain("if (operation === 'move')");
    expect(archive).toContain('ArchiveImportEntries');
  });

  it('keeps archive-to-archive disabled until a direct streaming bridge exists', () => {
    expect(dual).toContain('if (destination.contextState.isInArchive) return false;');
  });

  it('rewrites ZIPs transactionally instead of modifying them in place', () => {
    expect(binding).toContain('os.CreateTemp(filepath.Dir(resolvedArchive), ".archive-import-*.zip")');
    expect(binding).toContain('os.Rename(resolvedArchive, backup)');
    expect(binding).toContain('os.Rename(tmpPath, resolvedArchive)');
  });
});
