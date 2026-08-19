import { readFileSync } from 'node:fs';

describe('Registry delete elevation routing', () => {
  const api = readFileSync(
    new URL('./api.service.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('./plugin-manager.service.ts', import.meta.url),
    'utf8',
  );

  it('routes Registry delete through App.DeleteFile', () => {
    expect(api).toContain('window.go.app.App.DeleteFile(');
    expect(manager).toContain(
      'await this.api.deletePath(context.targetPath, true, false)',
    );
  });

  it('does not route Registry delete through the batch filesystem runner', () => {
    expect(manager).not.toContain(
      'this.api.batchDelete([context.targetPath], true, false)',
    );
  });
});
