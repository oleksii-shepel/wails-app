import { readFileSync } from 'node:fs';

describe('Selection Basket visibility', () => {
  const basket = readFileSync(
    new URL('./selection-basket.component.ts', import.meta.url),
    'utf8',
  );
  const app = readFileSync(new URL('../app.component.ts', import.meta.url), 'utf8');
  const menu = readFileSync(
    new URL('./application-menu-bar.component.ts', import.meta.url),
    'utf8',
  );

  it('invalidates its own view when opened or closed through the app overlay host', () => {
    expect(basket).toContain('private readonly cdr: ChangeDetectorRef');
    expect(basket).toMatch(/show\(\): void \{[\s\S]*this\.visible = true;[\s\S]*this\.cdr\.markForCheck\(\);/);
    expect(basket).toMatch(/hide\(\): void \{[\s\S]*this\.visible = false;[\s\S]*this\.cdr\.markForCheck\(\);/);
  });

  it('invalidates when basket contents change', () => {
    expect(basket).toMatch(/this\.entries = entries;[\s\S]*this\.cdr\.markForCheck\(\);/);
  });

  it('is app-owned and remains exposed from the Tools menu', () => {
    expect(app).toContain('<app-selection-basket');
    expect(app).toContain('showSelectionBasket(): void { this.selectionBasket?.show(); }');
    expect(menu).toContain("command(FILE_MANAGER_COMMANDS.openSelectionBasket, 'Selection Basket…')");
  });
});
