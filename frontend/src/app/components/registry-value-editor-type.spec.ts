import { readFileSync } from 'node:fs';

describe('Registry value editor type UI', () => {
  const editor = readFileSync(
    new URL('./registry-value-editor.component.ts', import.meta.url),
    'utf8',
  );

  it('does not expose value-name editing', () => {
    expect(editor).not.toContain('registry-value-name');
  });

  it('uses a Registry type dropdown', () => {
    expect(editor).toContain('<select');
    expect(editor).toContain('[(ngModel)]="type"');
    expect(editor).toContain("'REG_DWORD'");
    expect(editor).toContain("'REG_QWORD'");
    expect(editor).toContain("'REG_BINARY'");
  });
});
