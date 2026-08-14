import { describe, expect, it } from 'vitest';
import { normalizeDiscoveredPluginPackage } from './plugin-discovery.service';

describe('normalizeDiscoveredPluginPackage', () => {
  it('normalizes backend discovery diagnostics', () => {
    const value = normalizeDiscoveredPluginPackage({
      manifest: { schemaVersion: 1, id: 'epikode.hash-tools', name: 'Hash Tools', version: '1.0.0' },
      path: '/plugins/epikode.hash-tools',
      manifestPath: '/plugins/epikode.hash-tools/plugin.json',
      valid: true,
      warnings: ['directory mismatch'],
    });
    expect(value.manifest.id).toBe('epikode.hash-tools');
    expect(value.valid).toBe(true);
    expect(value.errors).toEqual([]);
    expect(value.warnings).toEqual(['directory mismatch']);
  });
});
