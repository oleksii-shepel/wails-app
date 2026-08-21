import { readFileSync } from 'node:fs';

const directory = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);
const registry = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);

function block(source: string, start: string, end: string): string {
  const from = source.indexOf(start);
  if (from < 0) throw new Error(`Missing block start: ${start}`);
  const to = source.indexOf(end, from);
  if (to < 0) throw new Error(`Missing block end: ${end}`);
  return source.slice(from, to + end.length);
}

describe('Registry pane Directory visual parity', () => {
  it('keeps Directory styles byte-for-byte identical', () => {
    const directoryStyles = block(directory, '  styles: [`', '  `]');
    const registryStyles = block(registry, '  styles: [`', '  `]');

    expect(registryStyles).toBe(directoryStyles);
  });

  it('keeps the same Directory row/cell structure and classes', () => {
    for (const token of [
      'class="content"',
      'class="entry"',
      'class="icon"',
      'class="name"',
      'class="kind"',
      'class="size"',
      'class="modified"',
      '[style.grid-template-columns]="viewMode === \'details\' ? detailsGridTemplate : null"',
    ]) {
      expect(registry).toContain(token);
      expect(directory).toContain(token);
    }
  });

  it('uses Registry-specific Details labels without adding new chrome', () => {
    expect(registry).toContain(
      '<span></span><span>Name</span><span>Kind</span><span>Type</span><span>Value</span>',
    );
    expect(registry).not.toContain('.registry-');
    expect(registry).not.toContain('class="registry-');
  });

  it('uses a key emoji for Registry values', () => {
    expect(registry).toContain("case 'Value':");
    expect(registry).toContain("return '\\\\u{1F511}';");
  });

  it('shows Registry value type and preview in the existing Directory columns', () => {
    expect(registry).toContain('{{ entryDisplayType(entry) }}');
    expect(registry).toContain('{{ entryDisplayValue(entry) }}');
    expect(registry).toContain("entry.registryValueType?.trim() || 'REG_SZ'");
    expect(registry).toContain("entry.registryPreview?.trim() || ''");
  });
});
