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
  it('keeps the Directory template byte-for-byte identical', () => {
    const directoryTemplate = block(directory, '  template: `', '  `,');
    const registryTemplate = block(registry, '  template: `', '  `,');

    expect(registryTemplate).toBe(directoryTemplate);
  });

  it('keeps the Directory styles byte-for-byte identical', () => {
    const directoryStyles = block(directory, '  styles: [`', '  `]');
    const registryStyles = block(registry, '  styles: [`', '  `]');

    expect(registryStyles).toBe(directoryStyles);
  });

  it('customizes only Registry entry semantics', () => {
    expect(registry).toContain("private registryEntryKind(entry: FileInfo)");
    expect(registry).toContain("entry.registryValueType?.trim() || 'Value'");
    expect(registry).toContain("entry.registryPreview?.trim() || ''");
    expect(registry).toContain("if (this.isRegistryHive(entry)) return 'Hive'");
  });

  it('does not add Registry-specific CSS or markup', () => {
    expect(registry).not.toContain('.registry-');
    expect(registry).not.toContain('class="registry-');
  });
});
