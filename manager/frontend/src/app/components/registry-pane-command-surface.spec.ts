import { readFileSync } from 'node:fs';

const registry = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);
const dual = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Registry leaf editing and command surfaces', () => {
  it('opens Registry value editor through the panel-source activation hook', () => {
    expect(registry).toContain('(click)="onRegistryEntryClick(entry, $event)"');
    expect(registry).toContain("if (this.registryEntryKind(entry) !== 'Value') return;");
    expect(registry).toContain('source.activateEntry?.(');
  });

  it('does not open Registry values in the file viewer', () => {
    const openEntry = registry.slice(
      registry.indexOf('openEntry(entry: FileInfo): void'),
      registry.indexOf('onEntryDragStart(', registry.indexOf('openEntry(entry: FileInfo): void')),
    );
    expect(openEntry).toContain("this.registryEntryKind(entry) === 'Value'");
    expect(openEntry.indexOf("this.registryEntryKind(entry) === 'Value'"))
      .toBeLessThan(openEntry.indexOf('this.openViewer(entry)'));
  });

  it('preserves DWORD and QWORD hexadecimal width inline', () => {
    expect(registry).toContain("type === 'REG_DWORD'");
    expect(registry).toContain("type === 'REG_QWORD'");
    expect(registry).toContain("padStart(width, '0')");
    expect(registry).toContain('const width = bits / 4;');
  });

  it('uses only Registry plugin actions in the Registry toolbar', () => {
    expect(registry).toContain('aria-label="Registry actions"');
    expect(registry).not.toContain('title="New folder (F7)"');
    expect(registry).not.toContain('title="Copy (Ctrl+C)"');
    expect(registry).not.toContain('title="Cut (Ctrl+X)"');
    expect(registry).not.toContain('title="Paste (Ctrl+V)"');
  });

  it('replaces generic file context menus with Registry plugin menus', () => {
    expect(dual).toContain("if (/^registry:\\/\\//i.test(pane?.contextState.path ?? ''))");
    expect(dual).toContain("title: entry.name || 'Registry entry'");
    expect(dual).toContain("title: pane?.contextState.path || 'Registry'");
  });
});
