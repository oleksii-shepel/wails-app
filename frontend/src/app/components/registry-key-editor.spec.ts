import { readFileSync } from 'node:fs';

describe('Registry key editor interaction', () => {
  const editor = readFileSync(
    new URL('./registry-key-editor.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('../services/plugin-manager.service.ts', import.meta.url),
    'utf8',
  );

  it('does not use generic prompt or trap directives', () => {
    expect(editor).not.toContain('appTrapFocus');
    expect(editor).not.toContain('ModalSurface');
    expect(manager).toContain("this.registryKeyEditor.open({");
  });

  it('owns its input directly', () => {
    expect(editor).toContain('data-input-owned');
    expect(editor).toContain('input.focus({ preventScroll: true })');
  });

  it('supports both create and rename', () => {
    expect(manager).toContain("title: 'New Registry Key'");
    expect(manager).toContain('title: `Edit Registry Key — ${info.name}`');
  });
});
