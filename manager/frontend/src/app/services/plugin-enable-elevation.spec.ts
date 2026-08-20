import { readFileSync } from 'node:fs';

describe('explicit plugin enable elevation', () => {
  const installation = readFileSync(
    new URL('./plugin-installation-state.service.ts', import.meta.url),
    'utf8',
  );

  it('persists enabled state before requesting host privilege', () => {
    const method = installation.slice(
      installation.indexOf('async setEnabled('),
      installation.indexOf('async setTrusted('),
    );
    expect(method.indexOf('this.update(pluginId, { enabled })'))
      .toBeLessThan(method.indexOf('EnsureUniversalPluginHostPrivilege(pluginId)'));
  });

  it('rolls back enablement when UAC/elevation fails', () => {
    const method = installation.slice(
      installation.indexOf('async setEnabled('),
      installation.indexOf('async setTrusted('),
    );
    expect(method).toContain('await this.update(pluginId, { enabled: false });');
    expect(method).toContain('throw error;');
  });

  it('does not request privilege when disabling', () => {
    const method = installation.slice(
      installation.indexOf('async setEnabled('),
      installation.indexOf('async setTrusted('),
    );
    expect(method).toContain('if (!enabled) return state;');
  });
});
