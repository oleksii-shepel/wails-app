export interface SourceCapabilities {
  readonly read: boolean;
  readonly create: boolean;
  readonly delete: boolean;
  readonly rename: boolean;
  readonly trash: boolean;
}

const READ_ONLY: SourceCapabilities = Object.freeze({
  read: true,
  create: false,
  delete: false,
  rename: true,
  trash: false,
});

const MUTABLE: SourceCapabilities = Object.freeze({
  read: true,
  create: true,
  delete: true,
  rename: true,
  trash: false,
});

const LOCAL: SourceCapabilities = Object.freeze({ ...MUTABLE, trash: true });
const REGISTRY: SourceCapabilities = Object.freeze({
  read: true,
  create: true,
  delete: true,
  rename: false,
  trash: false,
});

export function getSourceCapabilities(
  location: string,
  options?: { inArchive?: boolean },
): SourceCapabilities {
  // Archive mutation is handled by the archive operation layer rather than the
  // mounted VFS adapter, so keep those commands available while browsing it.
  if (options?.inArchive) return MUTABLE;

  const value = (location ?? '').trim();
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(value);
  if (!match) return LOCAL;

  switch (match[1].toLowerCase()) {
    case 'filesystem':
      return LOCAL;
    case 'memory':
    case 'smb':
      return MUTABLE;
    case 'registry':
      return REGISTRY;
    case 'gdrive':
    case 'dropbox':
    case 'onedrive':
    case 's3':
    case 'search':
    case 'http':
    case 'https':
      return READ_ONLY;
    default:
      // Unknown/plugin schemes are conservative until they advertise their own
      // source capabilities. Reading/copying out is safe; mutation is not.
      return READ_ONLY;
  }
}