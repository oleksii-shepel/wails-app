import type { ViewerFileListItem } from '../shared/protocol-enhanced';

export function archiveViewerSibling(
  fileList: readonly ViewerFileListItem[],
  currentPath: string,
  delta: -1 | 1,
): ViewerFileListItem | null {
  const normalize = (value: string) =>
    (value ?? '').replace(/\\/g, '/').replace(/^\/+|\/+$/g, '');

  const current = normalize(currentPath);
  const index = fileList.findIndex(item =>
    normalize(item.path) === current,
  );

  if (index < 0) return null;
  return fileList[index + delta] ?? null;
}
