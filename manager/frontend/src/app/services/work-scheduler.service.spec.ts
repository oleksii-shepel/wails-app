import { WorkSchedulerService } from './work-scheduler.service';

describe('WorkSchedulerService', () => {
  it('runs higher-priority queued work first', async () => {
    const scheduler = new WorkSchedulerService();
    const order: string[] = [];

    const blockers = Array.from({ length: 8 }, (_, index) =>
      scheduler.schedule({
        key: `block-${index}`,
        priority: 'interactive',
        kind: 'default',
        run: async context => {
          await context.yield('interactive');
          order.push(`block-${index}`);
        },
      }).promise,
    );

    const background = scheduler.schedule({
      key: 'background',
      priority: 'background',
      kind: 'default',
      run: () => { order.push('background'); },
    });

    const visible = scheduler.schedule({
      key: 'visible',
      priority: 'visible',
      kind: 'default',
      run: () => { order.push('visible'); },
    });

    await Promise.all([...blockers, visible.promise, background.promise]);
    expect(order.indexOf('visible')).toBeLessThan(order.indexOf('background'));
  });

  it('deduplicates equal work keys', async () => {
    const scheduler = new WorkSchedulerService();
    let calls = 0;

    const request = () => scheduler.schedule({
      key: 'metadata:/a',
      priority: 'interactive' as const,
      kind: 'metadata' as const,
      run: async context => {
        calls += 1;
        await context.yield();
        return 42;
      },
    });

    const first = request();
    const second = request();

    expect(first.id).toBe(second.id);
    expect(await first.promise).toBe(42);
    expect(await second.promise).toBe(42);
    expect(calls).toBe(1);
  });

  it('cancels an old scope generation', async () => {
    const scheduler = new WorkSchedulerService();
    const firstGeneration = scheduler.beginGeneration('pane:left');

    const first = scheduler.schedule({
      key: 'left:index:1',
      scope: 'pane:left',
      generation: firstGeneration,
      priority: 'background',
      kind: 'directory',
      run: async context => {
        await context.yield('background');
        return 'old';
      },
    });

    scheduler.beginGeneration('pane:left');

    await expectAsync(first.promise).toBeRejectedWith(
      jasmine.objectContaining({ name: 'AbortError' }),
    );
  });

  it('reports work progress centrally', async () => {
    const scheduler = new WorkSchedulerService();
    const progress: number[] = [];

    const handle = scheduler.schedule({
      priority: 'interactive',
      kind: 'search',
      run: context => {
        context.report({ completed: 1, total: 2 });
        context.report({ completed: 2, total: 2 });
        return 'done';
      },
    });

    handle.onProgress(value => progress.push(value.completed ?? 0));
    expect(await handle.promise).toBe('done');
    expect(progress).toContain(2);
  });
});