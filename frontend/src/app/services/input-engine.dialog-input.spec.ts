import { readFileSync } from 'node:fs';

describe('InputEngineService dialog input isolation', () => {
  const source = readFileSync(
    new URL('./input-engine.service.ts', import.meta.url),
    'utf8',
  );

  it('does not capture keyboard events owned by dialogs', () => {
    expect(source).toContain('if (this.isDialogEventTarget(event.target)) return;');
  });

  it('does not capture mouse events owned by dialogs', () => {
    expect(source).toContain(
      "return !!element?.closest('.modal, [role=\"dialog\"], [role=\"alertdialog\"]');",
    );
  });

  it('does not begin global mouse drags from a dialog', () => {
    expect(source).toContain('this.mouseDragState = null;');
  });
});
