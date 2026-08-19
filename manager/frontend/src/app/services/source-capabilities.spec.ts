import { getSourceCapabilities } from './source-capabilities';

describe('getSourceCapabilities', () => {
  it('allows full local mutation including trash', () => {
    expect(getSourceCapabilities('filesystem://c/Users')).toEqual({
      read: true, create: true, delete: true, rename: true, trash: true,
    });
  });

  it('keeps SMB mutable without claiming native trash support', () => {
    expect(getSourceCapabilities('smb://server/share')).toEqual(
      jasmine.objectContaining({
        create: true,
        delete: true,
        rename: true,
        trash: false,
      }),
    );
  });

  it('models Registry key/value rename support', () => {
    expect(getSourceCapabilities('registry://HKEY_CURRENT_USER/Software')).toEqual(
      jasmine.objectContaining({
        create: true,
        delete: true,
        rename: true,
      }),
    );
  });

  [
    'gdrive://me',
    'dropbox://me',
    'onedrive://me',
    's3://bucket',
    'search://?pattern=*.ts',
    'https://example.com',
  ].forEach((location) => {
    it(`${location} is read-only but readable`, () => {
      expect(getSourceCapabilities(location)).toEqual({
        read: true,
        create: false,
        delete: false,
        rename: false,
        trash: false,
      });
    });
  });

  it('keeps archive mutation available through the archive operation layer', () => {
    expect(getSourceCapabilities('filesystem://c/archive.zip/folder', { inArchive: true })).toEqual(
      jasmine.objectContaining({
        create: true,
        delete: true,
        rename: true,
      }),
    );
  });
});