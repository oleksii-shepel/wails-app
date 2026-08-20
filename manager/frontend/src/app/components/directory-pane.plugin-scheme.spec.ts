import { readFileSync } from 'node:fs';

describe('Directory pane strict plugin-scheme navigation', () => {
  const source = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('normalizes leading slash and bare scheme roots', () => {
    expect(source).toContain("/^\\/[a-z][a-z0-9+.-]*:/i.test(value)");
    expect(source).toContain('return `${scheme}://`;');
  });

  it('does not silently fall through plugin schemes to filesystem', () => {
    expect(source).toContain("resolved.source.kind === 'filesystem'");
    expect(source).toContain('No enabled plugin is registered for the ${scheme}:// scheme.');
    expect(source).toContain('no panel source registered the ${scheme}:// scheme.');
  });

  it('preserves the requested location when plugin activation fails', () => {
    expect(source).toContain('this.path = location;');
    expect(source).toContain('this.emitPathChange(location);');
    expect(source).toContain('this.error = error instanceof Error');
  });
});
