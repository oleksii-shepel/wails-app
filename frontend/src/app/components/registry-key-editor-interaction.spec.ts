import { readFileSync } from 'node:fs';

describe('Registry key editor interaction', () => {
  const source = readFileSync(
    new URL('./registry-key-editor.component.ts', import.meta.url),
    'utf8',
  );

  it('owns keyboard and pointer events locally', () => {
    expect(source).toContain('(keydown)="onDialogKeyDown($event)"');
    expect(source).toContain('(mousedown)="$event.stopPropagation()"');
    expect(source).toContain('(mouseup)="$event.stopPropagation()"');
  });

  it('does not depend on trap or modal directives', () => {
    expect(source).not.toContain('appTrapFocus');
    expect(source).not.toContain('ModalSurface');
  });

  it('closes directly on Escape', () => {
    expect(source).toContain("if (event.key === 'Escape')");
    expect(source).toContain('this.cancel()');
  });
});
