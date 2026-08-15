import '@angular/compiler';

import { canonicalizePathReference, normalizePath } from './address-bar.utils';

describe('address-bar alias normalization', () => {
  it('keeps s3 aliases in alias form', () => {
    expect(canonicalizePathReference('s3://minio-dev/photos/2026')).toBe('s3://minio-dev/photos/2026');
    expect(normalizePath('s3://minio-dev/photos/2026')).toBe('s3://minio-dev/photos/2026');
  });

  it('keeps smb aliases in alias form', () => {
    expect(canonicalizePathReference('smb://fileserver/share/docs')).toBe('smb://fileserver/share/docs');
    expect(normalizePath('smb://fileserver/share/docs')).toBe('smb://fileserver/share/docs');
  });

  it('keeps cloud aliases in alias form', () => {
    expect(canonicalizePathReference('onedrive://Projects/2026')).toBe('onedrive://Projects/2026');
    expect(normalizePath('gdrive://Shared/team')).toBe('gdrive://Shared/team');
    expect(normalizePath('dropbox://Photos/raw')).toBe('dropbox://Photos/raw');
  });

  it('keeps web aliases in alias form', () => {
    expect(canonicalizePathReference('https://example.com/assets/main.js')).toBe('https://example.com/assets/main.js');
    expect(normalizePath('https://example.com/assets/main.js')).toBe('https://example.com/assets/main.js');
  });

  it('converts unc paths to smb aliases without collapsing the scheme', () => {
    const uncPath = String.raw`\\fileserver\share\docs`;

    expect(canonicalizePathReference(uncPath)).toBe('smb://fileserver/share/docs');
    expect(normalizePath(uncPath)).toBe('smb://fileserver/share/docs');
  });
});

