import { readFileSync } from 'node:fs';

describe('Selection Basket batch workspace geometry', () => {
  const basket = readFileSync(new URL('./selection-basket.component.ts', import.meta.url), 'utf8');

  it('uses a centered, bounded batch workspace rather than a side drawer', () => {
    for (const fragment of [
      'width: min(940px, calc(100vw - 24px));',
      'height: min(700px, calc(100vh - 40px));',
      'border-radius: 8px;',
      'box-shadow: 0 18px 60px var(--vsc-scrim);',
      'grid-template-columns:minmax(0,1fr) 300px;',
    ]) {
      expect(basket).withContext(`Selection Basket: ${fragment}`).toContain(fragment);
    }
  });

  it('keeps compact list rows beside the operation panel', () => {
    expect(basket).not.toContain('justify-content:flex-end');
    expect(basket).not.toContain('border-left:1px solid var(--vsc-border-bright)');
    expect(basket).toContain('background:var(--vsc-list-hover-background');
    expect(basket).toContain('font-family:var(--vsc-font-family-mono');
    expect(basket).toContain('class="batch-panel"');
  });

  it('retains explicit change detection when visibility changes', () => {
    expect(basket).toContain('private readonly cdr: ChangeDetectorRef');
    expect(basket).toContain('this.cdr.markForCheck();');
  });
});
