import { readFileSync } from 'node:fs';

describe('Plugin settings operations', () => {
  const panel = readFileSync(
    new URL('./plugin-settings-panel.component.ts', import.meta.url),
    'utf8',
  );
  const coordinator = readFileSync(
    new URL('../services/plugin-lifecycle-coordinator.service.ts', import.meta.url),
    'utf8',
  );

  it('bounds plugin operations and refresh so Working cannot remain forever', () => {
    expect(panel).toContain('withTimeout(action(), 10_000');
    expect(panel).toContain('Plugin refresh timed out');
    expect(panel).toContain('this.busyIds.delete(id)');
  });

  it('does not recursively sync inside policy setters', () => {
    const trusted = coordinator.slice(
      coordinator.indexOf('async setTrusted('),
      coordinator.indexOf('async setGrantedPermissions('),
    );
    expect(trusted).not.toContain('this.sync()');
  });
});
