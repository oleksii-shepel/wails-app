const WINDOWS_DEVICE = /^(?:CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])(?:[. ]|$)/i;

/** Validate a single filename without treating it as a destination path. */
export function filenameProblem(name: string, windows = false): string | null {
  if (!name.trim()) return 'Enter a non-empty filename.';
  if (name === '.' || name === '..' || /[\\/\u0000]/.test(name)) {
    return 'A filename cannot contain path separators or refer to a parent folder.';
  }
  if (!windows) return null;
  if (/[<>:"|?*\u0001-\u001f]/.test(name)) return 'The name contains a character Windows does not allow.';
  if (/[ .]$/.test(name)) return 'A Windows filename cannot end with a space or period.';
  if (WINDOWS_DEVICE.test(name)) return 'The name is reserved by Windows.';
  return null;
}
