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
  | 'rootsDrives' | 'attributesPermissions' | 'trashBrowseRestore' | 'openWithChooser' | 'driveLifecycle' | 'transferUndo' | 'copyPathName' | 'duplicate' | 'links' | 'terminalHere' | 'reveal' | 'relativePath' | 'timestamps' | 'multiProperties' | 'driveCapacity' | 'newTemplates' | 'shortcuts' | 'multiChecksum' | 'fileUri' | 'sameTypeSelection' | 'moveSelectionFolder' | 'nameSafety';

export const BASIC_FILE_MANAGER_FEATURES: readonly BasicFileManagerFeature[] = [
  'navigation','backForward','refresh','copyCutPaste','copyMoveOtherPane',
  'rename','newFileFolder','trashDelete','permanentDelete','undoRedo',
  'properties','checksum','folderSize','openWithPreview','searchFilter',
  'sortViews','hiddenFiles','selection','tabs','pinsHistory','dragDrop',
  'archiveBrowseExtract','archiveCreate','rootsDrives','attributesPermissions','trashBrowseRestore','openWithChooser','driveLifecycle','transferUndo','copyPathName','duplicate','links','terminalHere','reveal','relativePath','timestamps','multiProperties','driveCapacity','newTemplates','shortcuts','multiChecksum','fileUri','sameTypeSelection','moveSelectionFolder','nameSafety',
];

export function missingBasicFileManagerFeatures(
  implemented: ReadonlySet<BasicFileManagerFeature>,
): BasicFileManagerFeature[] {
  return BASIC_FILE_MANAGER_FEATURES.filter(feature => !implemented.has(feature));
}


export const BASIC_FILE_MANAGER_BASELINE_VERSION = 1 as const;

/**
 * The Phase 63 audit verified that every feature in BASIC_FILE_MANAGER_FEATURES
 * has a concrete implementation in the reassembly shell. Advanced plugin,
 * compare/sync, remote-provider and viewer-specific features remain outside this
 * ordinary desktop file-manager baseline.
 */
export const BASIC_FILE_MANAGER_BASELINE_COMPLETE = true as const;
