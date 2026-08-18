import { readFileSync } from 'node:fs';

describe('Phase 74 build regressions', () => {
  const module = readFileSync(
    new URL('../plugin/declarative-plugin-module.ts', import.meta.url),
    'utf8',
  );
  const loader = readFileSync(
    new URL('../services/directory-plugin-package-loader.service.ts', import.meta.url),
    'utf8',
  );
  const engine = readFileSync(
    new URL('../services/input-engine.service.ts', import.meta.url),
    'utf8',
  );

  it('declares declarative plugin bindings in the module contract', () => {
    expect(module).toContain('export interface DeclarativePluginBinding');
    expect(module).toContain('bindings?: DeclarativePluginBinding[]');
  });

  it('types declarative binding mapping explicitly', () => {
    expect(loader).toContain(
      '(binding: DeclarativePluginBinding) =>',
    );
  });

  it('does not use an invalid broad string type predicate', () => {
    expect(engine).toContain(
      'value is Exclude<typeof value, undefined>',
    );
    expect(engine).not.toContain(
      "gesture.direction,\n    ].filter((value): value is string",
    );
  });
});
