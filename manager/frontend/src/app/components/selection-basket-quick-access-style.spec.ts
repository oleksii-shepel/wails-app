import { readFileSync } from 'node:fs';

describe('Selection Basket Quick Access visual parity', () => {
  const basket = readFileSync(new URL('./selection-basket.component.ts', import.meta.url), 'utf8');
  const quick = readFileSync(new URL('./quick-access.component.ts', import.meta.url), 'utf8');

  it('uses the Quick Access centered dialog geometry', () => {
    for (const fragment of [
      'place-items: start center;',
      'padding-top: min(10vh, 90px);',
      'width: min(720px, calc(100vw - 24px));',
      'max-height: min(680px, calc(100vh - 40px));',
      'border-radius: 8px;',
      'box-shadow: 0 18px 60px var(--vsc-scrim);',
    ]) {
      expect(quick).withContext(`Quick Access reference: ${fragment}`).toContain(fragment.replace(/: /g, ':'));
      expect(basket).withContext(`Selection Basket: ${fragment}`).toContain(fragment);
    }
  });

  it('uses compact list-row styling rather than the old side drawer', () => {
    expect(basket).not.toContain('justify-content:flex-end');
    expect(basket).not.toContain('height:100%');
    expect(basket).not.toContain('border-left:1px solid var(--vsc-border-bright)');
    expect(basket).toContain('background: var(--vsc-list-hover-background');
    expect(basket).toContain('font-family: var(--vsc-font-family-mono');
  });

  it('retains explicit change detection when visibility changes', () => {
    expect(basket).toContain('private readonly cdr: ChangeDetectorRef');
    expect(basket).toContain('this.cdr.markForCheck();');
  });
});
