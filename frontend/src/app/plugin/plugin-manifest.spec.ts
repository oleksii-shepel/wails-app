import { isPluginApiCompatible, validatePluginManifest, type PluginManifest } from './plugin-manifest';

const valid: PluginManifest = {
  schemaVersion: 1,
  id: 'epikode.hash-tools',
  name: 'Hash Tools',
  version: '1.2.0',
  apiVersion: '^1.0.0',
  permissions: ['commands'],
};

describe('plugin manifest', () => {
  it('accepts compatible manifests', () => {
    expect(validatePluginManifest(valid, '1.4.0').valid).toBe(true);
  });

  it('rejects incompatible API versions', () => {
    const result = validatePluginManifest({ ...valid, apiVersion: '^2.0.0' }, '1.4.0');
    expect(result.valid).toBe(false);
    expect(result.errors.join(' ')).toContain('requires plugin API');
  });

  it('supports intentionally small API range syntax', () => {
    expect(isPluginApiCompatible('1.x', '1.7.3')).toBe(true);
    expect(isPluginApiCompatible('^1.2.0', '1.8.0')).toBe(true);
    expect(isPluginApiCompatible('^1.2.0', '2.0.0')).toBe(false);
    expect(isPluginApiCompatible('1.2.3', '1.2.3')).toBe(true);
  });
});

