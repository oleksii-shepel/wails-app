import { readFileSync } from 'node:fs';

describe('Registry parent entry semantics', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('renders .. as Key on registry surfaces', () => {
    expect(pane).toContain(
      "if (this.isRegistrySurface && entry.path === PARENT_ENTRY_PATH) return 'Key';",
    );
    expect(pane).not.toContain(
      "entry.path === PARENT_ENTRY_PATH ? 'Folder' : entryDisplayKind(entry)",
    );
  });
});
