import { readFileSync } from 'node:fs';

describe('Confirm dialog interaction hardening', () => {
  const source = readFileSync(
    new URL('./confirm-dialog.component.ts', import.meta.url),
    'utf8',
  );

  it('owns a top-level interactive layer', () => {
    expect(source).toContain('z-index: 30000;');
    expect(source).toContain('pointer-events: auto;');
  });

  it('keeps panel clicks inside the dialog', () => {
    expect(source).toContain('(mousedown)="$event.stopPropagation()"');
    expect(source).toContain('(click)="$event.stopPropagation()"');
  });

  it('focuses the prompt and places the caret at the end', () => {
    expect(source).toContain('input.focus({ preventScroll: true })');
    expect(source).toContain('input.setSelectionRange(end, end)');
  });
});
