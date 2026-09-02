import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Details workspace synthetic parent handling', () => {
  it('resolves the synthetic parent row to a real filesystem path', () => {
    expect(source).toContain("const cursorIsParent = entry.path === '__parent__';");
    expect(source).toContain("this.parentLocation(pane.contextState.path)");
  });

  it('never stores __parent__ as the Details workspace folder path', () => {
    expect(source).toContain("if (!folderPath || folderPath === '__parent__') return;");
  });

  it('does not select the synthetic parent as an inspector item', () => {
    expect(source).toContain(
      "cursorIsParent || entry.type === FileType.DIRECTORY"
    );
  });
});
