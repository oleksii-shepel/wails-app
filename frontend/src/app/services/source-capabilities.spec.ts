import { describe, expect, it } from 'vitest';
import { getSourceCapabilities } from './source-capabilities';

describe('getSourceCapabilities', () => {
  it('allows full local mutation including trash', () => {
    expect(getSourceCapabilities('filesystem://c/Users')).toEqual({
      read: true, create: true, delete: true, rename: true, trash: true,
    });
  });

  it('keeps SMB mutable without claiming native trash support', () => {
    expect(getSourceCapabilities('smb://server/share')).toMatchObject({
      create: true, delete: true, rename: true, trash: false,
    });
  });

  it('models registry rename asymmetry', () => {
    expect(getSourceCapabilities('registry://HKEY_CURRENT_USER/Software')).toMatchObject({
      create: true, delete: true, rename: false,
    });
  });

  it.each(['gdrive://me', 'dropbox://me', 'onedrive://me', 's3://bucket', 'search://?pattern=*.ts', 'https://example.com'])('%s is read-only but readable', (location) => {
    expect(getSourceCapabilities(location)).toEqual({
      read: true, create: false, delete: false, rename: false, trash: false,
    });
  });

  it('keeps archive mutation available through the archive operation layer', () => {
    expect(getSourceCapabilities('filesystem://c/archive.zip/folder', { inArchive: true })).toMatchObject({
      create: true, delete: true, rename: true,
    });
  });
});
