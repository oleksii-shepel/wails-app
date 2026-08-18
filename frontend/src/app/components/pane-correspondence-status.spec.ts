import { readFileSync } from 'node:fs';

describe('Reassembly correspondence status integration', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('uses actual PaneComparisonStatus values', () => {
    expect(manager).toContain("case 'size-different':");
    expect(manager).toContain("case 'hash-different':");
    expect(manager).toContain("case 'content-different':");
    expect(manager).toContain("case 'type-different':");
    expect(manager).toContain("item.status !== 'same'");
  });
});
