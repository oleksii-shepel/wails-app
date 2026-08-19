import { readFileSync } from 'node:fs';

describe('ConfirmDialog native input contract', () => {
  const source = readFileSync(
    new URL('./confirm-dialog.component.ts', import.meta.url),
    'utf8',
  );

  it('uses a native form for prompt submission', () => {
    expect(source).toContain('(ngSubmit)="onConfirm()"');
    expect(source).toContain('type="submit"');
  });

  it('does not route prompt keydown through a component interceptor', () => {
    expect(source).not.toContain('(keydown)="onPromptKeyDown($event)"');
    expect(source).not.toContain('InputContextService');
    expect(source).not.toContain('InputEngineService');
  });

  it('keeps Escape local to the dialog surface', () => {
    expect(source).toContain('(keydown.escape)="onCancel(); $event.stopPropagation()"');
  });
});