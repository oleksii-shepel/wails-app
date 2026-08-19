import { readFileSync } from 'node:fs';

describe('Confirm dialog direct interaction', () => {
  const source = readFileSync(
    new URL('./confirm-dialog.component.ts', import.meta.url),
    'utf8',
  );

  it('does not use any focus/modal trap directive', () => {
    expect(source).not.toContain('ModalSurface');
    expect(source).not.toContain('appTrapFocus');
    expect(source).not.toContain('DialogFocusTrapDirective');
  });

  it('handles Escape directly', () => {
    expect(source).toContain('(keydown.escape)="onCancel(); $event.preventDefault(); $event.stopPropagation()"');
  });

  it('keeps native overlay freezing separate from focus management', () => {
    expect(source).toContain("this.overlayFreeze.acquire('dialog.confirm')");
    expect(source).toContain("this.overlayFreeze.release('dialog.confirm')");
  });
});
