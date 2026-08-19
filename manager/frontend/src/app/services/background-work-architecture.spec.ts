import { readFileSync } from 'node:fs';

describe('background work architecture', () => {
  const scheduler = readFileSync(
    new URL('./work-scheduler.service.ts', import.meta.url),
    'utf8',
  );
  const thumbnail = readFileSync(
    new URL('./thumbnail-pipeline.service.ts', import.meta.url),
    'utf8',
  );
  const discovery = readFileSync(
    new URL('./discovery-search.service.ts', import.meta.url),
    'utf8',
  );

  it('defines the five shared priority classes', () => {
    for (const priority of [
      "'visible'",
      "'interactive'",
      "'prefetch'",
      "'background'",
      "'idle'",
    ]) {
      expect(scheduler).toContain(priority);
    }
  });

  it('centralizes cancellation, concurrency, deduplication and progress', () => {
    expect(scheduler).toContain('beginGeneration(scope: string)');
    expect(scheduler).toContain('maxConcurrency = 8');
    expect(scheduler).toContain('this.keyed.get(dedupKey)');
    expect(scheduler).toContain('report(progress: WorkProgress)');
  });

  it('removes the thumbnail-specific queue and concurrency scheduler', () => {
    expect(thumbnail).toContain('WorkSchedulerService');
    expect(thumbnail).toContain("kind: 'thumbnail'");
    expect(thumbnail).not.toContain('private readonly queue');
    expect(thumbnail).not.toContain('private active =');
  });

  it('routes discovery provider requests and cooperative yielding through work scheduler', () => {
    expect(discovery).toContain("kind: 'provider'");
    expect(discovery).toContain("this.work.yield('interactive'");
    expect(discovery).not.toContain('new Promise<void>(resolve => setTimeout');
  });
});