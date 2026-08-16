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
  | 'rootsDrives' | 'attributesPermissions' | 'trashBrowseRestore' | 'openWithChooser' | 'driveLifecycle' | 'transferUndo' | 'copyPathName' | 'duplicate' | 'links' | 'terminalHere';

export const BASIC_FILE_MANAGER_FEATURES: readonly BasicFileManagerFeature[] = [
  'navigation','backForward','refresh','copyCutPaste','copyMoveOtherPane',
  'rename','newFileFolder','trashDelete','permanentDelete','undoRedo',
  'properties','checksum','folderSize','openWithPreview','searchFilter',
  'sortViews','hiddenFiles','selection','tabs','pinsHistory','dragDrop',
  'archiveBrowseExtract','archiveCreate','rootsDrives','attributesPermissions','trashBrowseRestore','openWithChooser','driveLifecycle','transferUndo','copyPathName','duplicate','links','terminalHere',
];

export function missingBasicFileManagerFeatures(
  implemented: ReadonlySet<BasicFileManagerFeature>,
): BasicFileManagerFeature[] {
  return BASIC_FILE_MANAGER_FEATURES.filter(feature => !implemented.has(feature));
}
