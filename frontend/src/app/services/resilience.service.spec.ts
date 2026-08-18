import { ResilienceService } from './resilience.service';

describe('ResilienceService', () => {
  it('retries a transient source failure without affecting another source', async () => {
    const service = new ResilienceService();
    let attempts = 0;

    const result = await service.execute(
      'provider:s3:a',
      async () => {
        attempts += 1;
        if (attempts < 2) throw new Error('temporary');
        return 42;
      },
      { policy: { maxAttempts: 2, initialDelayMs: 0, jitter: 0 } },
    );

    expect(result).toBe(42);
    expect(service.snapshot('provider:s3:a').state).toBe('online');
    expect(service.snapshot('provider:smb:b').state).toBe('online');
  });

  it('marks a source offline after terminal retry failure', async () => {
    const service = new ResilienceService();

    await expectAsync(service.execute(
      'provider:gdrive:x',
      async () => { throw new Error('unavailable'); },
      { policy: { maxAttempts: 1 } },
    )).toBeRejected();

    expect(service.snapshot('provider:gdrive:x').state).toBe('offline');
  });
});
