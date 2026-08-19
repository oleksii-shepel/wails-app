import { readFileSync } from 'node:fs';

describe('ConfirmDialog modal ownership', () => {
  const source = readFileSync(
    new URL('./confirm-dialog.component.ts', import.meta.url),
    'utf8',
  );

  it('uses the shared ModalSurface infrastructure', () => {
    expect(source).toContain('ModalSurface');
    expect(source).toContain('(ModalDismiss)="onCancel()"');
    expect(source).not.toContain('[appTrapFocus]="true"');
    expect(source).not.toContain('NativeOverlayFreezeService');
  });

  it('keeps prompt dialogs above context menus', () => {
    expect(source).toContain(
      'z-index: max(calc(var(--vsc-z-modal) + 19000), 21000);',
    );
  });

  it('lets the panel own pointer interaction while backdrop dismisses', () => {
    expect(source).toContain('(mousedown)="onCancel()"');
    expect(source).toContain('(mousedown)="$event.stopPropagation()"');
  });
});
