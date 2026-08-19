import { readFileSync } from 'node:fs';

describe('Registry value editor interaction', () => {
  const editor = readFileSync(
    new URL('./registry-value-editor.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('../services/plugin-manager.service.ts', import.meta.url),
    'utf8',
  );
  const input = readFileSync(
    new URL('../services/input-engine.service.ts', import.meta.url),
    'utf8',
  );

  it('uses a dedicated plain editor rather than ConfirmDialog prompt', () => {
    expect(manager).toContain('this.registryValueEditor.edit({');
    expect(editor).not.toContain('appTrapFocus');
    expect(editor).not.toContain('ModalSurface');
  });

  it('owns its mouse and keyboard input explicitly', () => {
    expect(editor).toContain('data-input-owned');
    expect(input).toContain("closest('[data-input-owned]')");
  });

  it('can close directly with Escape or buttons', () => {
    expect(editor).toContain("if (event.key !== 'Escape') return;");
    expect(editor).toContain('(click)="cancel()"');
  });
});
