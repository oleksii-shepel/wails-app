import { readFileSync } from 'node:fs';

describe('WorkspaceService registry restore contract', () => {
  const source = readFileSync(
    new URL('./workspace.service.ts', import.meta.url),
    'utf8',
  );

  it('prefers the durable location object over a stale path for directory tabs', () => {
    expect(source).toContain("const persistedPath = this.persistedNavigationTarget(tab, surfaceId);");
    expect(source).toContain("const location = tab.location?.location?.trim();");
    expect(source).toContain("if (location) {");
    expect(source).toContain("return location;");
  });
});
