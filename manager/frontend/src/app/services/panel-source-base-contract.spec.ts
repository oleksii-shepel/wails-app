import { readFileSync } from 'node:fs';

describe('BasePanelSource optional provider contract', () => {
  const source = readFileSync(
    new URL('./panel-source-registry.service.ts', import.meta.url),
    'utf8',
  );

  it('provides default activation and preview methods for subclasses', () => {
    expect(source).toContain('activateEntry(');
    expect(source).toContain('canPreviewEntry(');
  });
});
