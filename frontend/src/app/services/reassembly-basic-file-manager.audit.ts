export type BasicFileManagerFeature =
  | 'navigation' | 'backForward' | 'refresh'
  | 'copyCutPaste' | 'copyMoveOtherPane'
  | 'rename' | 'newFileFolder'
  | 'trashDelete' | 'permanentDelete' | 'undoRedo'
  | 'properties' | 'checksum' | 'folderSize'
  | 'openWithPreview' | 'searchFilter' | 'sortViews'
  | 'hiddenFiles' | 'selection' | 'tabs'
  | 'pinsHistory' | 'dragDrop'
  | 'archiveBrowseExtract' | 'archiveCreate'
  | 'rootsDrives' | 'attributesPermissions' | 'trashBrowseRestore' | 'openWithChooser' | 'driveLifecycle' | 'transferUndo';

export const BASIC_FILE_MANAGER_FEATURES: readonly BasicFileManagerFeature[] = [
  'navigation','backForward','refresh','copyCutPaste','copyMoveOtherPane',
  'rename','newFileFolder','trashDelete','permanentDelete','undoRedo',
  'properties','checksum','folderSize','openWithPreview','searchFilter',
  'sortViews','hiddenFiles','selection','tabs','pinsHistory','dragDrop',
  'archiveBrowseExtract','archiveCreate','rootsDrives','attributesPermissions','trashBrowseRestore','openWithChooser','driveLifecycle','transferUndo',
];

export function missingBasicFileManagerFeatures(
  implemented: ReadonlySet<BasicFileManagerFeature>,
): BasicFileManagerFeature[] {
  return BASIC_FILE_MANAGER_FEATURES.filter(feature => !implemented.has(feature));
}
