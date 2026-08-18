import { readFileSync } from 'node:fs';

describe('workspace resilience', () => {
  const source = readFileSync(
    new URL('./workspace.service.ts', import.meta.url),
    'utf8',
  );

  it('falls back to the durable local snapshot instead of recursively waiting for backend', () => {
    expect(source).toContain('workspaceList = this.loadLocalWorkspaces()');
    expect(source).not.toContain('return this.initializeWorkspaces()');
    expect(source).not.toContain('setTimeout(r => r');
  });

  it('does not persist huge viewer sibling populations', () => {
    expect(source).toContain('delete tab.viewer.fileList');
    expect(source).toContain('durableWorkspaceSnapshot');
  });

  it('keeps backend persistence best effort', () => {
    expect(source).toContain('.catch(() => undefined)');
  });
});
