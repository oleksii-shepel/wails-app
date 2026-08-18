import { readFileSync } from 'node:fs';

describe('recoverable transfer jobs', () => {
  const source = readFileSync(
    new URL('./power-file-operations.service.ts', import.meta.url),
    'utf8',
  );

  it('persists recoverable jobs but never treats an interrupted running job as completed', () => {
    expect(source).toContain('RECOVERY_KEY');
    expect(source).toContain('restoreRecoverableJobs');
    expect(source).toContain('Application restarted while this job was running.');
  });
});
