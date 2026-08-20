import { readFileSync } from 'node:fs';

describe('directory list pipeline regression guards', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const controller = readFileSync(
    new URL('../surfaces/dir/dir-list-stream.controller.ts', import.meta.url),
    'utf8',
  );

  it('rejects stale backend windows after navigation', () => {
    expect(pane).toContain('generation !== this.loadGeneration');
    expect(pane).toContain('streamId !== this.backendSessionId');
    expect(pane).toContain('requestPath !== this.path');
  });

  it('records explicit pipeline stages', () => {
    for (const stage of [
      'enumerateMs',
      'indexMs',
      'sortMs',
      'windowMs',
      'bridgeMs',
      'renderMs',
    ]) {
      expect(pane).toContain(stage);
    }
  });

  it('measures Wails bridge and backend window time separately', () => {
    expect(controller).toContain('bridgeMs: performance.now() - started');
    expect(controller).toContain('backendWindowMs');
  });
});