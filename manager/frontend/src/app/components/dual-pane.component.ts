import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  HostListener,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { ApiService } from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { WorkspaceService } from '../services/workspace.service';
import { WorkspaceSurfaceService } from '../services/workspace-surface.service';
import { WorkspaceSurfaceActionRegistryService } from '../services/workspace-surface-action-registry.service';
import { BuiltinWorkspaceSurfaceActionsService } from '../services/builtin-workspace-surface-actions.service';
import {
  splitLayout,
  surfaceLayout,
  workspaceSurface,
  type WorkspacePresentation,
  type WorkspaceSurface,
} from '../services/workspace-surface.model';
import { ToolWorkspaceHostComponent } from './tool-workspace-host.component';
import { PinnedLocationService } from '../services/pinned-location.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { canTransferBetween, transferBlockedMessage } from '../services/transfer-capabilities';
import { getSourceCapabilities } from '../services/source-capabilities';
import {
  PaneComparisonService,
  type PaneComparisonResult,
  type PaneComparisonPolicy,
} from '../services/pane-comparison.service';
import { PaneRecursiveComparisonService } from '../services/pane-recursive-comparison.service';
import { SelectionBasketService } from '../services/selection-basket.service';
import {
  BatchRenameDialogService,
  type BatchRenameItem,
} from '../services/batch-rename-dialog.service';
import { RemoteConnectDialogService } from '../services/remote-connect-dialog.service';
import { PaneHostComponent } from './pane-host.component';
import { WorkspaceSwitcherComponent } from './workspace-switcher.component';
import { PathHistoryComponent } from './path-history.component';
import { QuickAccessComponent, type QuickAccessTarget } from './quick-access.component';
import { RemoteConnectionsComponent } from './remote-connections.component';
import { OperationSurfaceComponent } from './operation-surface.component';
import { PaneComparisonBarComponent } from './pane-comparison-bar.component';
import {
  PaneSyncReviewComponent,
  type PaneSyncPlan,
} from './pane-sync-review.component';
import { SelectionBasketComponent } from './selection-basket.component';
import { ToolsCenterComponent } from './tools-center.component';
import {
  GlobalSearchComponent,
  type SearchResultsPanelRequest,
} from './global-search.component';
import {
  CommandPaletteComponent,
  type PaletteCommand,
} from './command-palette.component';
import {
  ContextMenuComponent,
  type ContextMenuItem,
  type ContextMenuState,
} from './context-menu.component';
import { normalizeContextMenuItems } from './context-menu-policy';
import { FileInfo, FileType } from '../shared/protocol';
import {
  collapseNestedDragEntries,
  decideInternalDrop,
  dragOperationFromModifiers,
} from './drag-drop-policy';
import { PluginManagerService } from '../services/plugin-manager.service';
import { OperationHistoryService, type SyncHistoryEntry } from '../services/operation-history.service';
import { TrashDialogService } from '../services/trash-dialog.service';
import { DriveLifecycleService } from '../services/drive-lifecycle.service';
import { PluginLifecycleCoordinatorService } from '../services/plugin-lifecycle-coordinator.service';
import { SettingsService } from '../services/settings.service';
import {
  PaneIntelligenceService,
  type PanePairPreset,
} from '../services/pane-intelligence.service';
import { SurfaceRegistryService } from '../surfaces/surface-registry.service';
import type { ToolbarContext, MenuContext } from '../plugin/plugin-registry-api';
import { adaptPluginToolbarActions } from './pane-toolbar-action';
import { presentPluginCommand } from './plugin-command-presentation';
import type { PresentedCommand } from './presented-command';
import { analyzePaneLocations, mirrorPaneLocation, mirroredPeerCandidates } from './pane-link-policy';
import type { PanePresentation } from './pane-component.contract';
import { FILE_MANAGER_COMMANDS } from './file-manager-command-registry';
import { InputCenterComponent } from './input-center.component';
import { KeyboardHelpComponent } from './keyboard-help.component';
import { OperationJournalComponent } from './operation-journal.component';
import { ClosedTabsComponent } from './closed-tabs.component';
import type { RecentlyClosedTab } from '../services/workspace.service';
import { InputCustomizationService } from '../services/input-customization.service';
import { PowerFileOperationsService } from '../services/power-file-operations.service';

type PaneId = 'left' | 'right';

const CONTEXT_ID = 'reassembly.manager';
const OWNER_ID = 'reassembly.manager';

const COMMAND = {
  toggleShortcutHelp: 'reassembly.help.shortcuts',
  showShortcutReference: FILE_MANAGER_COMMANDS.showKeyboardHelp,
  openCommandPalette: FILE_MANAGER_COMMANDS.openCommandPalette,
  openSettings: FILE_MANAGER_COMMANDS.openSettings,
  openPluginSettings: 'reassembly.settings.plugins',
  switchPane: FILE_MANAGER_COMMANDS.switchPane,
  togglePaneLink: 'reassembly.panes.toggleLink',
  togglePaneMirror: 'reassembly.panes.toggleMirror',
  togglePaneFilterMirror: 'reassembly.panes.toggleFilterMirror',
  togglePaneScrollSync: 'reassembly.panes.toggleScrollSync',
  togglePaneFollow: 'reassembly.panes.toggleFollow',
  matchOtherPane: 'reassembly.panes.matchOther',
  swapPaneRoots: 'reassembly.panes.swapRoots',
  openPeer: 'reassembly.panes.openPeer',
  copyToPeer: 'reassembly.panes.copyToPeer',
  moveToPeer: 'reassembly.panes.moveToPeer',
  previousDifference: 'reassembly.panes.previousDifference',
  nextDifference: 'reassembly.panes.nextDifference',
  savePanePreset: 'reassembly.panes.savePreset',
  applyPanePreset: 'reassembly.panes.applyPreset',
  widenLeftPane: 'reassembly.layout.widenLeft',
  widenRightPane: 'reassembly.layout.widenRight',
  resetPaneSplit: 'reassembly.layout.resetSplit',
  cursorUp: FILE_MANAGER_COMMANDS.cursorUp,
  cursorDown: FILE_MANAGER_COMMANDS.cursorDown,
  open: FILE_MANAGER_COMMANDS.openItem,
  up: FILE_MANAGER_COMMANDS.navigateUp,
  back: 'reassembly.manager.back',
  forward: 'reassembly.manager.forward',
  location: 'reassembly.manager.location',
  selectAll: FILE_MANAGER_COMMANDS.selectAll,
  selectSameExtension: FILE_MANAGER_COMMANDS.selectSameExt,
  selectSameType: 'reassembly.selection.sameType',
  clearSelection: FILE_MANAGER_COMMANDS.deselectAll,
  refresh: FILE_MANAGER_COMMANDS.refresh,
  rename: FILE_MANAGER_COMMANDS.rename,
  newFile: 'reassembly.manager.newFile',
  newTextDocument: 'reassembly.manager.newTextDocument',
  newMarkdownDocument: 'reassembly.manager.newMarkdownDocument',
  newJsonDocument: 'reassembly.manager.newJsonDocument',
  createShortcut: 'reassembly.files.createShortcut',
  newFolder: FILE_MANAGER_COMMANDS.newFolder,
  moveSelectionToNewFolder: 'reassembly.manager.moveSelectionToNewFolder',
  properties: 'reassembly.manager.properties',
  detailsWorkspace: 'reassembly.workspace.details',
  deletePermanent: 'reassembly.manager.deletePermanent',
  deleteSelected: FILE_MANAGER_COMMANDS.deleteSelected,
  copy: FILE_MANAGER_COMMANDS.copy,
  cut: FILE_MANAGER_COMMANDS.cut,
  paste: FILE_MANAGER_COMMANDS.paste,
  copyPath: 'reassembly.files.copyPath',
  copyRelativePath: 'reassembly.files.copyRelativePath',
  copyFileUri: 'reassembly.files.copyFileUri',
  copyName: 'reassembly.files.copyName',
  revealInFileManager: 'reassembly.files.revealInFileManager',
  setModifiedTime: 'reassembly.files.setModifiedTime',
  duplicate: 'reassembly.files.duplicate',
  createSymlink: 'reassembly.files.createSymlink',
  createHardlink: 'reassembly.files.createHardlink',
  openTerminal: 'reassembly.navigation.openTerminal',
  undo: 'reassembly.edit.undo',
  redo: 'reassembly.edit.redo',
  operationHistory: 'reassembly.edit.operationHistory',
  createArchive: 'reassembly.archive.createZip',
  checksum: 'reassembly.files.checksum',
  splitFile: 'reassembly.files.splitFile',
  joinFiles: 'reassembly.files.joinFiles',
  calculateSize: 'reassembly.files.calculateSize',
  attributes: 'reassembly.files.attributes',
  openTrash: 'reassembly.navigation.trash',
  copyToOther: FILE_MANAGER_COMMANDS.copyToOtherPane,
  moveToOther: FILE_MANAGER_COMMANDS.moveToOtherPane,
  newTab: FILE_MANAGER_COMMANDS.newTab,
  closeTab: FILE_MANAGER_COMMANDS.closeTab,
  reopenClosedTab: 'reassembly.manager.reopenClosedTab',
  showClosedTabs: 'reassembly.manager.showClosedTabs',
  nextTab: FILE_MANAGER_COMMANDS.nextTab,
  previousTab: FILE_MANAGER_COMMANDS.prevTab,
  toggleTabPin: FILE_MANAGER_COMMANDS.togglePinTab,
  moveTabToOtherPane: 'reassembly.manager.moveTabToOtherPane',
  moveTabLeft: 'reassembly.manager.moveTabLeft',
  moveTabRight: 'reassembly.manager.moveTabRight',
  closeOtherTabs: 'reassembly.manager.closeOtherTabs',
  closeTabsToRight: 'reassembly.manager.closeTabsToRight',
  jumpTab1: FILE_MANAGER_COMMANDS.jumpToTab(1),
  jumpTab2: FILE_MANAGER_COMMANDS.jumpToTab(2),
  jumpTab3: FILE_MANAGER_COMMANDS.jumpToTab(3),
  jumpTab4: FILE_MANAGER_COMMANDS.jumpToTab(4),
  jumpTab5: FILE_MANAGER_COMMANDS.jumpToTab(5),
  jumpTab6: FILE_MANAGER_COMMANDS.jumpToTab(6),
  jumpTab7: FILE_MANAGER_COMMANDS.jumpToTab(7),
  jumpTab8: FILE_MANAGER_COMMANDS.jumpToTab(8),
  jumpTab9: FILE_MANAGER_COMMANDS.jumpToTab(9),
  viewerPrevious: 'reassembly.viewer.previous',
  viewerNext: 'reassembly.viewer.next',
  viewerZoomIn: 'reassembly.viewer.zoomIn',
  viewerZoomOut: 'reassembly.viewer.zoomOut',
  viewerFit: 'reassembly.viewer.fit',
  viewerActualSize: 'reassembly.viewer.actualSize',
  viewerReturnSource: 'reassembly.viewer.returnSource',
  viewerOpenExternal: 'reassembly.viewer.openExternal',
  openAsText: 'reassembly.viewer.openAsText',
  openAsBinary: 'reassembly.viewer.openAsBinary',
  openAsImage: 'reassembly.viewer.openAsImage',
  openAsMarkdown: 'reassembly.viewer.openAsMarkdown',
  openAsPdf: 'reassembly.viewer.openAsPdf',
  openAsLegacy: 'reassembly.viewer.openAsLegacy',
  closeContextMenu: 'reassembly.manager.closeContextMenu',
  focusFilter: FILE_MANAGER_COMMANDS.focusFilter,
  toggleHidden: FILE_MANAGER_COMMANDS.toggleHidden,
  showWorkspaceSwitcher: FILE_MANAGER_COMMANDS.showWorkspaceSwitcher,
  previousWorkspace: 'reassembly.workspace.previous',
  nextWorkspace: 'reassembly.workspace.next',
  newWorkspace: FILE_MANAGER_COMMANDS.newWorkspace,
  showPathHistory: FILE_MANAGER_COMMANDS.showPathHistory,
  showQuickAccess: 'reassembly.navigation.quickAccess',
  toggleCurrentPin: 'reassembly.navigation.toggleCurrentPin',
  showPinnedLocations: 'reassembly.navigation.pinnedLocations',
  treeLeft: FILE_MANAGER_COMMANDS.cursorLeft,
  treeRight: FILE_MANAGER_COMMANDS.cursorRight,
  comparePanes: FILE_MANAGER_COMMANDS.comparePanes,
  compareWorkspace: 'reassembly.workspace.compare',
  recursiveComparePanes: FILE_MANAGER_COMMANDS.recursiveComparePanes,
  reviewSyncPlan: FILE_MANAGER_COMMANDS.reviewSyncPlan,
  selectLeftDifferences: FILE_MANAGER_COMMANDS.selectLeftDifferences,
  selectRightDifferences: FILE_MANAGER_COMMANDS.selectRightDifferences,
  copyMissingLeftToRight: FILE_MANAGER_COMMANDS.copyMissingLeftToRight,
  copyMissingRightToLeft: FILE_MANAGER_COMMANDS.copyMissingRightToLeft,
  refreshComparison: 'reassembly.compare.refresh',
  addLeftDifferencesToBasket: 'reassembly.compare.basketLeft',
  addRightDifferencesToBasket: 'reassembly.compare.basketRight',
  addAllDifferencesToBasket: 'reassembly.compare.basketAll',

  toggleCurrentSelection: FILE_MANAGER_COMMANDS.toggleCurrent,
  extendSelectionUp: FILE_MANAGER_COMMANDS.extendSelUp,
  extendSelectionDown: FILE_MANAGER_COMMANDS.extendSelDown,
  selectByMask: FILE_MANAGER_COMMANDS.selectByMask,
  unselectByMask: FILE_MANAGER_COMMANDS.unselectByMask,
  invertSelection: FILE_MANAGER_COMMANDS.invertSelection,
  selectFiles: 'reassembly.selection.files',
  selectFolders: 'reassembly.selection.folders',
  unselectFiles: 'reassembly.selection.unselectFiles',
  unselectFolders: 'reassembly.selection.unselectFolders',
  cursorHome: FILE_MANAGER_COMMANDS.cursorHome,
  cursorEnd: FILE_MANAGER_COMMANDS.cursorEnd,
  cursorPageUp: FILE_MANAGER_COMMANDS.cursorPageUp,
  cursorPageDown: FILE_MANAGER_COMMANDS.cursorPageDown,

  openBasket: FILE_MANAGER_COMMANDS.openSelectionBasket,
  addToBasket: FILE_MANAGER_COMMANDS.addToSelectionBasket,
  clearBasket: FILE_MANAGER_COMMANDS.clearSelectionBasket,

  openOperations: FILE_MANAGER_COMMANDS.openOperations,
  diskUsage: FILE_MANAGER_COMMANDS.diskUsage,
  findDuplicates: FILE_MANAGER_COMMANDS.findDuplicates,
  recentFiles: 'reassembly.tools.recentFiles',
  batchRename: FILE_MANAGER_COMMANDS.batchRename,

  globalSearch: FILE_MANAGER_COMMANDS.openGlobalSearch,
  quickPathJump: FILE_MANAGER_COMMANDS.quickPathJump,
  quickView: FILE_MANAGER_COMMANDS.quickView,
  openWithExternal: FILE_MANAGER_COMMANDS.openWithExternal,
  openWithChooser: 'reassembly.files.openWithChooser',
  extractArchiveSelection: 'reassembly.archive.extractSelection',
  extractArchiveToOther: 'reassembly.archive.extractToOther',
  remoteConnect: 'reassembly.navigation.remoteConnect',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.manager.reopenClosedTab.ctrlShiftT', commandId:COMMAND.reopenClosedTab, contextId:CONTEXT_ID, source:'default', description:'Reopen last closed tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'t', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.edit.operationHistory.ctrlShiftH', commandId:COMMAND.operationHistory, contextId:CONTEXT_ID, source:'default', description:'Open operation history', category:'Edit', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.mouse.file.doubleClick', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open selected item with double-click', category:'Mouse', when:'fileEntry && !viewerSurface', trigger:{device:'mouse', gesture:{type:'doubleClick', button:'left'}}},
  { id:'reassembly.mouse.viewer.ctrlWheelUp', commandId:COMMAND.viewerZoomIn, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer in with Ctrl+Wheel', category:'Mouse', when:'viewerSurface', preventDefault:true, trigger:{device:'mouse', gesture:{type:'wheel', modifiers:['ctrl'], direction:'up'}}},
  { id:'reassembly.mouse.viewer.ctrlWheelDown', commandId:COMMAND.viewerZoomOut, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer out with Ctrl+Wheel', category:'Mouse', when:'viewerSurface', preventDefault:true, trigger:{device:'mouse', gesture:{type:'wheel', modifiers:['ctrl'], direction:'down'}}},
  { id:'reassembly.panes.link.ctrlAltL', commandId:COMMAND.togglePaneLink, contextId:CONTEXT_ID, source:'default', description:'Toggle linked pane navigation', category:'Pane Intelligence', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'l', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.panes.mirror.ctrlAltM', commandId:COMMAND.togglePaneMirror, contextId:CONTEXT_ID, source:'default', description:'Toggle pane mirroring', category:'Pane Intelligence', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'m', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.panes.follow.ctrlAltF', commandId:COMMAND.togglePaneFollow, contextId:CONTEXT_ID, source:'default', description:'Toggle follow selection', category:'Pane Intelligence', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.panes.scroll.ctrlAltS', commandId:COMMAND.togglePaneScrollSync, contextId:CONTEXT_ID, source:'default', description:'Toggle synchronized scrolling', category:'Pane Intelligence', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'s', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.panes.previousDifference.ctrlAltBracketLeft', commandId:COMMAND.previousDifference, contextId:CONTEXT_ID, source:'default', description:'Previous pane difference', category:'Pane Intelligence', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'[', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.panes.nextDifference.ctrlAltBracketRight', commandId:COMMAND.nextDifference, contextId:CONTEXT_ID, source:'default', description:'Next pane difference', category:'Pane Intelligence', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:']', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.layout.widenLeft.altShiftLeft', commandId:COMMAND.widenLeftPane, contextId:CONTEXT_ID, source:'default', description:'Widen left pane', category:'Layout', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft', modifiers:['alt','shift']}]}}},
  { id:'reassembly.layout.widenRight.altShiftRight', commandId:COMMAND.widenRightPane, contextId:CONTEXT_ID, source:'default', description:'Widen right pane', category:'Layout', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright', modifiers:['alt','shift']}]}}},
  { id:'reassembly.layout.resetSplit.altShift0', commandId:COMMAND.resetPaneSplit, contextId:CONTEXT_ID, source:'default', description:'Reset pane split', category:'Layout', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'0', modifiers:['alt','shift']}]}}},
  { id:'reassembly.help.shortcutReference.f1', commandId:COMMAND.showShortcutReference, contextId:'global', source:'default', priority:1000, description:'Show shortcut reference', category:'Help', trigger:{device:'keyboard', chord:{steps:[{key:'f1'}]}}},
  { id:'reassembly.help.inputCenter.ctrlAltK', commandId:COMMAND.toggleShortcutHelp, contextId:'global', source:'default', description:'Open Input & Shortcuts', category:'Help', trigger:{device:'keyboard', chord:{steps:[{key:'k', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.files.deletePermanent.shiftDelete', commandId:COMMAND.deletePermanent, contextId:CONTEXT_ID, source:'default', description:'Delete permanently', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete', modifiers:['shift']}]}}},
  { id:'reassembly.files.properties.altEnter', commandId:COMMAND.properties, contextId:CONTEXT_ID, source:'default', description:'Properties', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'enter', modifiers:['alt']}]}}},
  { id:'reassembly.navigation.commandPalette.ctrlShiftP', commandId:COMMAND.openCommandPalette, contextId:'global', source:'default', description:'Open command palette', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'p', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.settings.open.ctrlComma', commandId:COMMAND.openSettings, contextId:CONTEXT_ID, source:'default', description:'Open Settings', category:'Settings', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:',', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.switchPane.tab', commandId:COMMAND.switchPane, contextId:CONTEXT_ID, source:'default', description:'Switch active pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'tab'}]}}},
  { id:'reassembly.manager.cursorUp.arrowUp', commandId:COMMAND.cursorUp, contextId:CONTEXT_ID, source:'default', description:'Move cursor up', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.manager.cursorDown.arrowDown', commandId:COMMAND.cursorDown, contextId:CONTEXT_ID, source:'default', description:'Move cursor down', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.manager.open.enter', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open selected item', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
  { id:'reassembly.manager.up.backspace', commandId:COMMAND.up, contextId:CONTEXT_ID, source:'default', description:'Parent directory', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'backspace'}]}}},
  { id:'reassembly.manager.back.altLeft', commandId:COMMAND.back, contextId:CONTEXT_ID, source:'default', description:'Back', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft', modifiers:['alt']}]}}},
  { id:'reassembly.manager.forward.altRight', commandId:COMMAND.forward, contextId:CONTEXT_ID, source:'default', description:'Forward', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright', modifiers:['alt']}]}}},
  { id:'reassembly.manager.location.ctrlL', commandId:COMMAND.location, contextId:CONTEXT_ID, source:'default', description:'Edit location', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'l', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.selectAll.ctrlA', commandId:COMMAND.selectAll, contextId:CONTEXT_ID, source:'default', description:'Select all', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'a', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.clearSelection.ctrlAltD', commandId:COMMAND.clearSelection, contextId:CONTEXT_ID, source:'default', description:'Clear selection', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'d', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.manager.refresh.ctrlR', commandId:COMMAND.refresh, contextId:CONTEXT_ID, source:'default', description:'Refresh active pane', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'r', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.rename.f2', commandId:COMMAND.rename, contextId:CONTEXT_ID, source:'default', description:'Rename selected item', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f2'}]}}},
  { id:'reassembly.manager.newFolder.f7', commandId:COMMAND.newFolder, contextId:CONTEXT_ID, source:'default', description:'Create folder', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f7'}]}}},
  { id:'reassembly.manager.delete.delete', commandId:COMMAND.deleteSelected, contextId:CONTEXT_ID, source:'default', description:'Delete selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete'}]}}},
  { id:'reassembly.manager.delete.f8', commandId:COMMAND.deleteSelected, contextId:CONTEXT_ID, source:'default', description:'Delete selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f8'}]}}},
  { id:'reassembly.manager.copy.ctrlC', commandId:COMMAND.copy, contextId:CONTEXT_ID, source:'default', description:'Copy selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.cut.ctrlX', commandId:COMMAND.cut, contextId:CONTEXT_ID, source:'default', description:'Cut selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'x', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.paste.ctrlV', commandId:COMMAND.paste, contextId:CONTEXT_ID, source:'default', description:'Paste items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'v', modifiers:['ctrl']}]}}},
  { id:'reassembly.files.copyPath.ctrlShiftC', commandId:COMMAND.copyPath, contextId:CONTEXT_ID, source:'default', description:'Copy selected paths as text', category:'Files', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.files.copyRelativePath.ctrlAltC', commandId:COMMAND.copyRelativePath, contextId:CONTEXT_ID, source:'default', description:'Copy selected relative paths', category:'Files', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.selection.sameExtension.ctrlAltE', commandId:COMMAND.selectSameExtension, contextId:CONTEXT_ID, source:'default', description:'Select files with the same extension', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'e', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.files.duplicate.ctrlD', commandId:COMMAND.duplicate, contextId:CONTEXT_ID, source:'default', description:'Duplicate selection', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'d', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.openTerminal.ctrlAltT', commandId:COMMAND.openTerminal, contextId:CONTEXT_ID, source:'default', description:'Open terminal in current folder', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'t', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.edit.undo.ctrlZ', commandId:COMMAND.undo, contextId:CONTEXT_ID, source:'default', description:'Undo last reversible file operation', category:'Edit', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'z', modifiers:['ctrl']}]}}},
  { id:'reassembly.edit.redo.ctrlY', commandId:COMMAND.redo, contextId:CONTEXT_ID, source:'default', description:'Redo last reversible file operation', category:'Edit', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'y', modifiers:['ctrl']}]}}},
  { id:'reassembly.edit.redo.ctrlShiftZ', commandId:COMMAND.redo, contextId:CONTEXT_ID, source:'default', description:'Redo last reversible file operation', category:'Edit', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'z', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.archive.createZip.altF5', commandId:COMMAND.createArchive, contextId:CONTEXT_ID, source:'default', description:'Create ZIP archive from selection', category:'Archive', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f5', modifiers:['alt']}]}}},
  { id:'reassembly.manager.copyToOther.f5', commandId:COMMAND.copyToOther, contextId:CONTEXT_ID, source:'default', description:'Copy to other pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f5'}]}}},
  { id:'reassembly.manager.moveToOther.f6', commandId:COMMAND.moveToOther, contextId:CONTEXT_ID, source:'default', description:'Move to other pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f6'}]}}},
  { id:'reassembly.manager.newTab.ctrlT', commandId:COMMAND.newTab, contextId:CONTEXT_ID, source:'default', description:'New tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'t', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.closeTab.ctrlW', commandId:COMMAND.closeTab, contextId:CONTEXT_ID, source:'default', description:'Close tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'w', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.nextTab.ctrlTab', commandId:COMMAND.nextTab, contextId:CONTEXT_ID, source:'default', description:'Next tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'tab', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.previousTab.ctrlShiftTab', commandId:COMMAND.previousTab, contextId:CONTEXT_ID, source:'default', description:'Previous tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'tab', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.manager.jumpTab1.ctrl1', commandId:COMMAND.jumpTab1, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 1', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'1', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab2.ctrl2', commandId:COMMAND.jumpTab2, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 2', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'2', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab3.ctrl3', commandId:COMMAND.jumpTab3, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 3', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'3', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab4.ctrl4', commandId:COMMAND.jumpTab4, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 4', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'4', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab5.ctrl5', commandId:COMMAND.jumpTab5, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 5', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'5', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab6.ctrl6', commandId:COMMAND.jumpTab6, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 6', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'6', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab7.ctrl7', commandId:COMMAND.jumpTab7, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 7', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'7', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab8.ctrl8', commandId:COMMAND.jumpTab8, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 8', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'8', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.jumpTab9.ctrl9', commandId:COMMAND.jumpTab9, contextId:CONTEXT_ID, source:'default', description:'Jump to tab 9', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'9', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.toggleTabPin.ctrlAltP', commandId:COMMAND.toggleTabPin, contextId:CONTEXT_ID, source:'default', description:'Pin or unpin current tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'p', modifiers:['ctrl','alt']}]}}},
  { id:'reassembly.viewer.previous.left', commandId:COMMAND.viewerPrevious, contextId:CONTEXT_ID, source:'default', description:'Previous viewer item', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft'}]}}},
  { id:'reassembly.viewer.next.right', commandId:COMMAND.viewerNext, contextId:CONTEXT_ID, source:'default', description:'Next viewer item', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright'}]}}},
  { id:'reassembly.viewer.zoomIn.ctrlPlus', commandId:COMMAND.viewerZoomIn, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer in', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'+', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.zoomIn.ctrlEquals', commandId:COMMAND.viewerZoomIn, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer in', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'=', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.zoomOut.ctrlMinus', commandId:COMMAND.viewerZoomOut, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer out', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'-', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.fit.ctrl0', commandId:COMMAND.viewerFit, contextId:CONTEXT_ID, source:'default', description:'Fit viewer to window', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'0', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.actualSize.ctrlAlt1', commandId:COMMAND.viewerActualSize, contextId:CONTEXT_ID, source:'default', description:'Viewer actual size', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'1', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.manager.contextMenu.escape', commandId:COMMAND.closeContextMenu, contextId:CONTEXT_ID, source:'default', description:'Close context menu', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}, preventDefault:true, stopPropagation:true },
  { id:'reassembly.manager.focusFilter.ctrlF', commandId:COMMAND.focusFilter, contextId:CONTEXT_ID, source:'default', description:'Focus folder filter', category:'Search & Filter', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f', modifiers:['ctrl']}]}}},
  { id:'reassembly.workspace.toggleHidden.ctrlShiftH', commandId:COMMAND.toggleHidden, contextId:CONTEXT_ID, source:'default', description:'Show/hide hidden files', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.workspace.showSwitcher.ctrlShiftW', commandId:COMMAND.showWorkspaceSwitcher, contextId:CONTEXT_ID, source:'default', description:'Open workspace switcher', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'w', modifiers:['ctrl','shift']}]}}},
{ id:'reassembly.workspace.previous.ctrlAltLeft', commandId:COMMAND.previousWorkspace, contextId:CONTEXT_ID, source:'default', description:'Previous workspace', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft', modifiers:['alt','ctrl']}]}}},
{ id:'reassembly.workspace.next.ctrlAltRight', commandId:COMMAND.nextWorkspace, contextId:CONTEXT_ID, source:'default', description:'Next workspace', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright', modifiers:['alt','ctrl']}]}}},
  { id:'reassembly.workspace.new.ctrlShiftAltN', commandId:COMMAND.newWorkspace, contextId:CONTEXT_ID, source:'default', description:'Create workspace', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'n', modifiers:['ctrl','shift','alt']}]}}},
  { id:'reassembly.navigation.pathHistory.ctrlH', commandId:COMMAND.showPathHistory, contextId:CONTEXT_ID, source:'default', description:'Show path history', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.quickAccess.ctrlQ', commandId:COMMAND.showQuickAccess, contextId:CONTEXT_ID, source:'default', description:'Open Quick Access', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'q', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.pinnedLocations.ctrlB', commandId:COMMAND.showPinnedLocations, contextId:CONTEXT_ID, source:'default', description:'Open Quick Access pinned section', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'b', modifiers:['ctrl']}]}}},
  { id:'reassembly.tree.left.arrowLeft', commandId:COMMAND.treeLeft, contextId:CONTEXT_ID, source:'default', description:'Tree collapse or parent', category:'Navigation', when:'!typing && !viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft'}]}}},
  { id:'reassembly.tree.right.arrowRight', commandId:COMMAND.treeRight, contextId:CONTEXT_ID, source:'default', description:'Tree expand or child', category:'Navigation', when:'!typing && !viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright'}]}}},
  { id:'reassembly.compare.toggle.altShiftC', commandId:COMMAND.comparePanes, contextId:CONTEXT_ID, source:'default', description:'Compare panes', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['alt','shift']}]}}},
  { id:'reassembly.compare.recursive.ctrlShiftAltC', commandId:COMMAND.recursiveComparePanes, contextId:CONTEXT_ID, source:'default', description:'Recursive pane compare', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['ctrl','shift','alt']}]}}},

  { id:'reassembly.selection.toggle.insert', commandId:COMMAND.toggleCurrentSelection, contextId:CONTEXT_ID, source:'default', description:'Toggle current selection', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'insert'}]}}},
  { id:'reassembly.selection.extendUp.shiftUp', commandId:COMMAND.extendSelectionUp, contextId:CONTEXT_ID, source:'default', description:'Extend selection upward', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup', modifiers:['shift']}]}}},
  { id:'reassembly.selection.extendDown.shiftDown', commandId:COMMAND.extendSelectionDown, contextId:CONTEXT_ID, source:'default', description:'Extend selection downward', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown', modifiers:['shift']}]}}},
  { id:'reassembly.selection.mask.altS', commandId:COMMAND.selectByMask, contextId:CONTEXT_ID, source:'default', description:'Select by mask', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'s', modifiers:['alt']}]}}},
  { id:'reassembly.selection.unmask.altU', commandId:COMMAND.unselectByMask, contextId:CONTEXT_ID, source:'default', description:'Unselect by mask', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'u', modifiers:['alt']}]}}},
  { id:'reassembly.selection.invert.altI', commandId:COMMAND.invertSelection, contextId:CONTEXT_ID, source:'default', description:'Invert selection', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'i', modifiers:['alt']}]}}},
  { id:'reassembly.selection.sameExtension.altE', commandId:COMMAND.selectSameExtension, contextId:CONTEXT_ID, source:'default', description:'Select same extension', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'e', modifiers:['alt']}]}}},
  { id:'reassembly.selection.files.altShiftF', commandId:COMMAND.selectFiles, contextId:CONTEXT_ID, source:'default', description:'Select visible files', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f', modifiers:['alt','shift']}]}}},
  { id:'reassembly.selection.folders.altShiftD', commandId:COMMAND.selectFolders, contextId:CONTEXT_ID, source:'default', description:'Select visible folders', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'d', modifiers:['alt','shift']}]}}},
  { id:'reassembly.selection.home.home', commandId:COMMAND.cursorHome, contextId:CONTEXT_ID, source:'default', description:'Move cursor to first item', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'home'}]}}},
  { id:'reassembly.selection.end.end', commandId:COMMAND.cursorEnd, contextId:CONTEXT_ID, source:'default', description:'Move cursor to last item', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'end'}]}}},
  { id:'reassembly.selection.pageUp.pageUp', commandId:COMMAND.cursorPageUp, contextId:CONTEXT_ID, source:'default', description:'Page up', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'pageup'}]}}},
  { id:'reassembly.selection.pageDown.pageDown', commandId:COMMAND.cursorPageDown, contextId:CONTEXT_ID, source:'default', description:'Page down', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'pagedown'}]}}},

  { id:'reassembly.basket.open.ctrlShiftB', commandId:COMMAND.openBasket, contextId:CONTEXT_ID, source:'default', description:'Open Selection Basket', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'b', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.basket.add.ctrlAltB', commandId:COMMAND.addToBasket, contextId:CONTEXT_ID, source:'default', description:'Add selection to basket', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'b', modifiers:['ctrl','alt']}]}}},

  { id:'reassembly.tools.diskUsage.ctrlShiftD', commandId:COMMAND.diskUsage, contextId:CONTEXT_ID, source:'default', description:'Analyze disk usage', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'d', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.files.batchRename.ctrlShiftR', commandId:COMMAND.batchRename, contextId:CONTEXT_ID, source:'default', description:'Batch rename selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'r', modifiers:['ctrl','shift']}]}}},

  { id:'reassembly.navigation.globalSearch.ctrlK', commandId:COMMAND.globalSearch, contextId:CONTEXT_ID, source:'default', description:'Open global search', category:'Search & Filter', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'k', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.quickPathJump.ctrlP', commandId:COMMAND.quickPathJump, contextId:CONTEXT_ID, source:'default', description:'Quick path jump', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'p', modifiers:['ctrl']}]}}},
  { id:'reassembly.files.quickView.f3', commandId:COMMAND.quickView, contextId:CONTEXT_ID, source:'default', description:'Quick view selected file', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f3'}]}}},
  { id:'reassembly.files.openWithExternal.ctrlShiftO', commandId:COMMAND.openWithExternal, contextId:CONTEXT_ID, source:'default', description:'Open with external application', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'o', modifiers:['ctrl','shift']}]}}},
];

@Component({
  selector: 'app-dual-pane',
  standalone: true,
  imports: [
    CommonModule,
    PaneHostComponent,
    ContextMenuComponent,
    InputCenterComponent,
    KeyboardHelpComponent,
    WorkspaceSwitcherComponent,
    PathHistoryComponent,
    QuickAccessComponent,
    RemoteConnectionsComponent,
    OperationSurfaceComponent,
    PaneComparisonBarComponent,
    PaneSyncReviewComponent,
    SelectionBasketComponent,
    ToolsCenterComponent,
    GlobalSearchComponent,
    ToolWorkspaceHostComponent,

    CommandPaletteComponent,    OperationJournalComponent,
    ClosedTabsComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      #managerRoot
      class="manager"
      role="application"
      [attr.data-workspace-kind]="workspacePresentation?.kind ?? 'files'"
      [attr.data-workspace-lifetime]="workspacePresentation?.lifetime ?? 'persistent'"
      aria-label="Dual-pane file manager"
      aria-describedby="manager-status"
      [style.grid-template-columns]="layoutColumns">
      <div id="manager-status" class="sr-only" role="status" aria-live="polite">
        Active pane {{ activePaneId }}. Left pane {{ leftPanePercent | number:'1.0-0' }} percent.
      </div>

      <app-tool-workspace-host
        *ngIf="workspacePresentation?.kind === 'tool' && workspacePresentation as toolWorkspace"
        class="tool-workspace-host"
        [presentation]="toolWorkspace"
        (close)="closeToolWorkspace()"
        (pin)="pinToolWorkspace()"
        (surfaceAction)="handleWorkspaceSurfaceAction($event)">
      </app-tool-workspace-host>

      <app-pane-comparison-bar
        *ngIf="workspacePresentation?.kind !== 'tool'"
        class="comparison-surface"
        [visible]="comparisonVisible"
        [leftEntries]="leftPane?.contextState.entries ?? []"
        [rightEntries]="rightPane?.contextState.entries ?? []"
        [leftPath]="leftPane?.contextState.path ?? ''"
        [rightPath]="rightPane?.contextState.path ?? ''"
        [recursive]="comparisonRecursive"
        [scanning]="recursiveComparisonScanning"
        [error]="recursiveComparisonError"
        [comparisonResult]="paneComparisonResult"
        [policy]="comparisonPolicy"
        (policyChange)="setComparisonPolicy($event)"
        (toggleRecursive)="toggleRecursiveComparison()"
        (refreshRecursive)="refreshRecursiveComparison()"
        (selectLeft)="selectComparisonDifferences('left')"
        (selectRight)="selectComparisonDifferences('right')"
        (copyMissingLeftToRight)="copyMissingComparison('left')"
        (copyMissingRightToLeft)="copyMissingComparison('right')"
        (review)="openSyncReview()"
        (close)="closePaneComparison()">
      </app-pane-comparison-bar>

      <app-pane-sync-review
        *ngIf="workspacePresentation?.kind !== 'tool'"
        class="comparison-surface"
        [visible]="syncReviewVisible"
        [result]="paneComparisonResult"
        [history]="recentSyncRuns"
        (execute)="executeSyncPlan($event)"
        (close)="closeSyncReview()">
      </app-pane-sync-review>
      <div
        class="pane-intelligence"
        *ngIf="workspacePresentation?.kind !== 'tool'"
        [class.linked]="linkedNavigation"
        [class.same]="paneLocationRelation === 'same'">
        <span class="relation" [title]="paneRelationTitle">
          {{ paneRelationLabel }}
        </span>

        <span class="link-status" *ngIf="linkStatus">
          {{ linkStatus }}
        </span>

        <span class="difference-count" *ngIf="differenceCount">
          {{ differenceIndexLabel }}
        </span>

        <span class="spacer"></span>

        <span class="live-modes" role="group" aria-label="Pane intelligence modes">
          <button
            type="button"
            class="mode-toggle"
            [class.active]="linkedNavigation"
            [attr.aria-pressed]="linkedNavigation"
            title="Linked navigation · Ctrl+Alt+L"
            (click)="toggleLinkedNavigation()">
            Link
          </button>
          <button
            type="button"
            class="mode-toggle"
            [class.active]="mirrorPresentation"
            [attr.aria-pressed]="mirrorPresentation"
            title="Mirror view/sort state · Ctrl+Alt+M"
            (click)="toggleMirrorPresentation()">
            Mirror
          </button>
          <button
            type="button"
            class="mode-toggle"
            [class.active]="syncScroll"
            [attr.aria-pressed]="syncScroll"
            title="Synchronized scrolling · Ctrl+Alt+S"
            (click)="toggleSyncScroll()">
            Scroll
          </button>
          <button
            type="button"
            class="mode-toggle"
            [class.active]="followSelection"
            [attr.aria-pressed]="followSelection"
            title="Follow matching item · Ctrl+Alt+F"
            (click)="toggleFollowSelection()">
            Follow
          </button>
        </span>

        <button
          #paneIntelligenceMore
          type="button"
          class="intelligence-more"
          [class.active]="paneIntelligenceMenuOpen"
          [attr.aria-expanded]="paneIntelligenceMenuOpen"
          aria-haspopup="menu"
          title="More pane actions"
          (click)="togglePaneIntelligenceMenu($event)">
          ⋯
        </button>

        <div
          #paneIntelligenceMenu
          *ngIf="paneIntelligenceMenuOpen"
          class="pane-intelligence-menu"
          role="menu"
          aria-label="Pane intelligence actions"
          (keydown)="onPaneIntelligenceMenuKeyDown($event)"
          (click)="$event.stopPropagation()">
          <section>
            <header>Relationship</header>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!canMatchOtherPane"
              (click)="runPaneIntelligenceAction('match')">Match other pane</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!canSwapLinkedRoots"
              (click)="runPaneIntelligenceAction('swapRoots')">Swap linked roots</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!canOpenCurrentCounterpart"
              (click)="runPaneIntelligenceAction('peer')">Open peer item</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!canCopyCurrentToPeer"
              (click)="runPaneIntelligenceAction('copyPeer')">Copy to other pane</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!canMoveCurrentToPeer"
              (click)="runPaneIntelligenceAction('movePeer')">Move to other pane</button>
          </section>

          <section>
            <header>Comparison</header>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="differenceCount === 0"
              (click)="runPaneIntelligenceAction('prevDiff')">Previous difference</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="differenceCount === 0"
              (click)="runPaneIntelligenceAction('nextDiff')">Next difference</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!mirrorPresentation"
              [class.checked]="mirrorFilter"
              (click)="runPaneIntelligenceAction('filter')">
              Mirror filter
              <span aria-hidden="true">{{ mirrorFilter ? '✓' : '' }}</span>
            </button>
          </section>

          <section>
            <header>Presets</header>
            <button type="button" role="menuitem" data-pane-intelligence-item
              (click)="runPaneIntelligenceAction('savePreset')">Save current pair…</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="panePairPresets.length === 0"
              (click)="runPaneIntelligenceAction('applyPreset')">Apply preset…</button>
            <button type="button" role="menuitem" data-pane-intelligence-item [disabled]="!selectedPanePresetId"
              (click)="runPaneIntelligenceAction('deletePreset')">Delete selected preset</button>
          </section>
        </div>
      </div>

      <div class="pane-slot" *ngIf="workspacePresentation?.kind !== 'tool'">
        <app-pane-host
          #leftPane
          paneId="left"
          [active]="activePaneId === 'left'"
          [showHidden]="showHidden"
          [comparisonStatuses]="leftComparisonStatuses"
          (activateRequested)="activate('left')"
          (locationChange)="onLinkedPaneLocationChange('left', $event)"
          (cursorChange)="onLinkedPaneCursorChange('left', $event)"
          (presentationChange)="onPanePresentationChange('left', $event)"
          (viewportChange)="onPaneViewportChange('left', $event)"
          (entryContextMenu)="openEntryMenu('left', $event.event, $event.entry)"
          (backgroundContextMenu)="openBackgroundMenu('left', $event)"
          (copyToOther)="activate('left'); copyToOther()"
          (moveToOther)="activate('left'); moveToOther()">
        </app-pane-host>
      </div>

      <div
        *ngIf="workspacePresentation?.kind !== 'tool'"
        class="divider"
        role="separator"
        tabindex="0"
        aria-label="Resize file panes"
        aria-orientation="vertical"
        [attr.aria-valuenow]="leftPanePercent | number:'1.0-0'"
        [attr.aria-valuemin]="MIN_PANE_PERCENT"
        [attr.aria-valuemax]="MAX_PANE_PERCENT"
        title="Drag to resize · Alt+Shift+Left/Right · Alt+Shift+0 reset"
        (pointerdown)="beginResize($event)"
        (pointermove)="resizeFromPointer($event)"
        (pointerup)="endResize($event)"
        (pointercancel)="endResize($event)"
        (dblclick)="resetSplit()">
        <span class="divider-grip" aria-hidden="true"></span>
      </div>

      <div class="pane-slot" *ngIf="workspacePresentation?.kind !== 'tool'">
        <app-pane-host
          #rightPane
          paneId="right"
          [active]="activePaneId === 'right'"
          [showHidden]="showHidden"
          [comparisonStatuses]="rightComparisonStatuses"
          (activateRequested)="activate('right')"
          (locationChange)="onLinkedPaneLocationChange('right', $event)"
          (cursorChange)="onLinkedPaneCursorChange('right', $event)"
          (presentationChange)="onPanePresentationChange('right', $event)"
          (viewportChange)="onPaneViewportChange('right', $event)"
          (entryContextMenu)="openEntryMenu('right', $event.event, $event.entry)"
          (backgroundContextMenu)="openBackgroundMenu('right', $event)"
          (copyToOther)="activate('right'); copyToOther()"
          (moveToOther)="activate('right'); moveToOther()">
        </app-pane-host>
      </div>

      <app-command-palette
        #commandPalette
        [commands]="commandPaletteCommands">
      </app-command-palette>

      <app-global-search
        #globalSearch
        (openResult)="openSearchResult($event)"
        (openContainingFolder)="openSearchContainingFolder($event)"
        (openAsPanel)="openSearchResultsPanel($event)"
        (openAsWorkspace)="openSearchWorkspace($event)"
        (openPanelPath)="openDiscoveryPanelPath($event)">
      </app-global-search>

      <app-tools-center
        #toolsCenter
        (changed)="refreshBothPanes()"
        (navigateTo)="navigateFromTools($event)"
        (openBasketRequested)="selectionBasket?.show()">
      </app-tools-center>

      <app-selection-basket
        #selectionBasket
        (copyTo)="copyBasketTo($event)"
        (moveTo)="moveBasketTo($event)"
        (deleteRequested)="deleteBasket()">
      </app-selection-basket>

      <app-workspace-switcher
        #workspaceSwitcher
        (workspaceActivated)="applyActiveWorkspace()"
        (toolWorkspaceActivated)="activateToolWorkspace($event)">
      </app-workspace-switcher>

      <app-path-history
        #pathHistory
        [paneId]="activePaneId"
        [currentPath]="activePane?.contextState.path ?? ''"
        (navigate)="navigateFromHistory($event)">
      </app-path-history>

      <app-quick-access
        #quickAccess
        [paneId]="activePaneId"
        [currentPath]="activePane?.contextState.path ?? ''"
        [currentSurfaceId]="activePane?.contextState.activeSurfaceId || 'dir'"
        (navigate)="navigateFromQuickAccess($event)"
        (openTarget)="openQuickAccessTarget($event)">
      </app-quick-access>


      <app-remote-connections
        #remoteConnections
        (navigate)="navigateFromRemoteConnections($event)">
      </app-remote-connections>

      <app-operation-surface
        (openDetails)="openTools('operations')">
      </app-operation-surface>

      <app-context-menu
        [state]="contextMenu"
        (command)="dispatchContextCommand($event)"
        (closed)="closeContextMenu()">
      </app-context-menu>

      <app-closed-tabs
        [visible]="closedTabsVisible"
        (closed)="closedTabsVisible = false"
        (reopenRequested)="reopenClosedTabEntry($event)">
      </app-closed-tabs>

      <app-operation-journal
        [visible]="operationJournalVisible"
        (closed)="operationJournalVisible = false"
        (undoRequested)="undoLastOperation()"
        (redoRequested)="redoLastOperation()">
      </app-operation-journal>

      <app-keyboard-help #keyboardHelp></app-keyboard-help>

      <app-input-center
        [visible]="shortcutHelpVisible"
        (closed)="shortcutHelpVisible = false">
      </app-input-center>
    </section>
  `,
  styles: [`
    :host {
      display: block;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      container-type: inline-size;
    }

    .sr-only{
      position:absolute;width:1px;height:1px;padding:0;margin:-1px;
      overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0;
    }

    .manager {
      display: grid;
      grid-template-rows: auto auto minmax(0, 1fr);
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }

    .tool-workspace-host {
      grid-column: 1 / -1;
      grid-row: 1 / -1;
      min-width: 0;
      min-height: 0;
      z-index: 5;
    }

    .comparison-surface {
      grid-column: 1 / -1;
      min-width: 0;
    }

    .pane-intelligence{
      position:relative;
      grid-column:1 / -1;
      display:flex;
      align-items:center;
      min-height:26px;
      gap:6px;
      padding:2px 6px;
      border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));
      background:var(--vsc-sideBar-background,var(--vsc-panel-background));
      color:var(--vsc-description-foreground,var(--vsc-foreground-dim));
      font:10px/1.2 var(--vsc-font-family,system-ui,sans-serif);
    }
    .pane-intelligence.linked{
      background:color-mix(in srgb,var(--vsc-accent-blue) 5%,var(--vsc-sideBar-background,var(--vsc-panel-background)));
    }
    .pane-intelligence.same .relation{color:var(--vsc-success)}
    .pane-intelligence .relation{
      max-width:220px;
      overflow:hidden;
      text-overflow:ellipsis;
      white-space:nowrap;
      font-weight:600;
      color:var(--vsc-foreground);
    }
    .pane-intelligence .link-status{
      min-width:0;
      max-width:280px;
      overflow:hidden;
      text-overflow:ellipsis;
      white-space:nowrap;
    }
    .pane-intelligence .difference-count{
      min-width:30px;
      text-align:center;
      font-variant-numeric:tabular-nums;
      color:var(--vsc-description-foreground,var(--vsc-foreground-dim));
    }
    .pane-intelligence .spacer{flex:1}
    .pane-intelligence .live-modes{
      display:inline-flex;
      align-items:center;
      gap:2px;
    }
    .pane-intelligence button{
      min-height:20px;
      padding:1px 7px;
      border:1px solid transparent;
      border-radius:3px;
      background:transparent;
      color:inherit;
      cursor:pointer;
      font:inherit;
    }
    .pane-intelligence button:hover:not(:disabled){
      border-color:var(--vsc-border);
      background:var(--vsc-list-hover-background);
      color:var(--vsc-foreground);
    }
    .pane-intelligence button.active{
      border-color:color-mix(in srgb,var(--vsc-accent-blue) 52%,var(--vsc-border));
      background:color-mix(in srgb,var(--vsc-accent-blue) 10%,transparent);
      color:var(--vsc-foreground);
    }
    .pane-intelligence button:disabled{
      opacity:.4;
      cursor:default;
    }
    .pane-intelligence .intelligence-more{
      width:26px;
      padding:0;
      font-size:15px;
      line-height:16px;
    }
    .pane-intelligence-menu{
      position:absolute;
      z-index:120;
      top:calc(100% + 2px);
      right:5px;
      width:220px;
      padding:5px;
      border:1px solid var(--vsc-border);
      border-radius:5px;
      background:var(--vsc-menu-background,var(--vsc-panel-background));
      color:var(--vsc-menu-foreground,var(--vsc-foreground));
      box-shadow:0 7px 22px var(--vsc-scrim-soft);
    }
    .pane-intelligence-menu section + section{
      margin-top:4px;
      padding-top:4px;
      border-top:1px solid var(--vsc-border-subtle,var(--vsc-border));
    }
    .pane-intelligence-menu header{
      padding:3px 6px;
      color:var(--vsc-description-foreground,var(--vsc-foreground-dim));
      font-size:9px;
      font-weight:600;
      text-transform:uppercase;
      letter-spacing:.04em;
    }
    .pane-intelligence-menu button{
      display:flex;
      width:100%;
      align-items:center;
      justify-content:space-between;
      min-height:24px;
      padding:3px 7px;
      text-align:left;
      color:inherit;
    }
    .pane-intelligence-menu button.checked{
      color:var(--vsc-foreground);
    }

    .pane-intelligence-menu button:focus-visible{
      outline:1px solid var(--vsc-focus-border,var(--vsc-accent-blue));
      outline-offset:-1px;
      background:var(--vsc-list-focus-background,var(--vsc-list-hover-background));
      color:var(--vsc-foreground);
    }

    .pane-slot {

      height: 100%;

      width: 100%;

      display: flex;

      grid-row: 3;

      min-width: 0;

      min-height: 0;

      overflow: hidden;

    }

    .pane-slot > app-pane-host {

      display: flex;

      flex: 1 1 0%;

      width: 100%;

      height: 100%;

      min-width: 0;

      min-height: 0;

      overflow: hidden;

    }


    .divider {
      grid-row: 3;
      position: relative;
      touch-action: none;
      cursor: col-resize;
      background:
        linear-gradient(
          90deg,
          transparent,
          var(--vsc-border, var(--vsc-control-background)) 40%,
          var(--vsc-border, var(--vsc-control-background)) 60%,
          transparent
        );
    }

    .divider:hover,
    .divider.resizing {
      background:
        linear-gradient(
          90deg,
          transparent,
          var(--vsc-accent-blue, var(--vsc-accent-blue)) 35%,
          var(--vsc-accent-blue, var(--vsc-accent-blue)) 65%,
          transparent
        );
    }

    .divider-grip {
      position: absolute;
      inset: 0 -3px;
    }

    @media (max-width: 820px) {
      .manager {
        grid-template-columns: 1fr !important;
        grid-template-rows: auto auto minmax(0, 1fr) 5px minmax(0, 1fr);
      }

      .pane-slot {
        grid-row: auto;
      }

      .divider {
        grid-row: auto;
        cursor: default;
        background:
          linear-gradient(
            180deg,
            transparent,
            var(--vsc-border, var(--vsc-control-background)) 40%,
            var(--vsc-border, var(--vsc-control-background)) 60%,
            transparent
          );
      }

      .divider-grip {
        display: none;
      }
    }
    .divider:focus-visible{
      outline:2px solid var(--vsc-focus-border,var(--vsc-accent-blue));
      outline-offset:-2px;
    }
    @container (max-width:760px){
      .divider{width:7px}
      .pane-slot{overflow:hidden}
    }
  `],
})
export class DualPaneComponent implements OnInit, OnDestroy {
  @ViewChild('managerRoot') managerRoot?: ElementRef<HTMLElement>;
  @ViewChild('paneIntelligenceMore') paneIntelligenceMore?: ElementRef<HTMLButtonElement>;
  @ViewChild('paneIntelligenceMenu') paneIntelligenceMenu?: ElementRef<HTMLElement>;
  @ViewChild('workspaceSwitcher')
  workspaceSwitcher?: WorkspaceSwitcherComponent;
  @ViewChild('selectionBasket')
  selectionBasket?: SelectionBasketComponent;
  @ViewChild('toolsCenter')
  toolsCenter?: ToolsCenterComponent;
  @ViewChild('commandPalette')
  commandPalette?: CommandPaletteComponent;
  @ViewChild('keyboardHelp')
  keyboardHelp?: KeyboardHelpComponent;
@ViewChild('globalSearch')
  globalSearchOverlay?: GlobalSearchComponent;
  @ViewChild('pathHistory')
  pathHistory?: PathHistoryComponent;
  @ViewChild('quickAccess')
  quickAccess?: QuickAccessComponent;
  @ViewChild('leftPane') leftPane?: PaneHostComponent;
  @ViewChild('rightPane') rightPane?: PaneHostComponent;
  @ViewChild('remoteConnections') remoteConnections?: RemoteConnectionsComponent;

  activePaneId: PaneId = 'left';
  readonly MIN_PANE_PERCENT = 20;
  readonly MAX_PANE_PERCENT = 80;

  leftPanePercent = 50;
  resizing = false;
  private workspaceApplyGeneration = 0;
  showHidden = false;
  shortcutHelpVisible = false;
  operationJournalVisible = false;
  closedTabsVisible = false;
  workspacePresentation: WorkspacePresentation | null = null;

  commandPaletteCommands: PaletteCommand[] = [];

  comparisonVisible = false;
  syncReviewVisible = false;
  comparisonRecursive = false;
  comparisonPolicy: PaneComparisonPolicy = 'size-date';
  verifiedComparisonResult: PaneComparisonResult | null = null;
  recursiveComparisonScanning = false;
  recursiveComparisonError = '';
  recursiveComparisonResult: PaneComparisonResult | null = null;
  recursiveComparisonRoots: { left: string; right: string } | null = null;

  lastTransfer = 'none';
  lastDrop = 'none';
  linkedNavigation = false;
  followSelection = false;
  mirrorPresentation = false;
  mirrorFilter = false;
  syncScroll = false;
  linkStatus = '';
  private scrollSyncGuard = false;
  private differenceCursor = -1;
  private paneIntelligenceEpoch = 0;
  private comparisonSignature = '';
  private recoveringPaneIntelligence = false;
  selectedPanePresetId = '';
  paneIntelligenceMenuOpen = false;
  private paneIntelligenceTypeahead = '';
  private paneIntelligenceTypeaheadTimer: ReturnType<typeof setTimeout> | null = null;
  private restoringPaneIntelligence = false;
  private linkedRoots: { left: string; right: string } | null = null;
  private linkedNavigationGuard = 0;
  private linkedSelectionGuard = false;
  contextMenu: ContextMenuState | null = null;
  private contextMenuReturnFocus: HTMLElement | null = null;
  private navigationTrailCommands = new Map<string, { paneId: PaneId; path: string }>();
  private contextMenuActions = new Map<string, () => void | Promise<void>>();
  private contextMenuActionSequence = 0;

  constructor(
    private readonly input: InputEngineService,
    private readonly inputCustomization: InputCustomizationService,
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly workspaceService: WorkspaceService,
    private readonly workspaceSurfaces: WorkspaceSurfaceService,
    private readonly workspaceSurfaceActions:
      WorkspaceSurfaceActionRegistryService,
    private readonly builtinWorkspaceSurfaceActions:
      BuiltinWorkspaceSurfaceActionsService,
    private readonly pinnedLocations: PinnedLocationService,
    private readonly paneComparison: PaneComparisonService,
    private readonly recursivePaneComparison: PaneRecursiveComparisonService,
    private readonly selectionBasketService: SelectionBasketService,
    private readonly batchRenameDialog: BatchRenameDialogService,
    private readonly remoteConnectDialog: RemoteConnectDialogService,
    private readonly pluginManager: PluginManagerService,
    private readonly operationHistory: OperationHistoryService,
    private readonly powerFileOperations: PowerFileOperationsService,
    private readonly trashDialog: TrashDialogService,
    private readonly driveLifecycle: DriveLifecycleService,
    private readonly pluginLifecycle: PluginLifecycleCoordinatorService,
    private readonly settings: SettingsService,
    private readonly paneIntelligence: PaneIntelligenceService,
    private readonly surfaceRegistry: SurfaceRegistryService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  @HostListener('document:mousedown', ['$event'])
  onDocumentMouseDown(event: Event): void {
    if (!this.paneIntelligenceMenuOpen) return;

    const target = event.target as HTMLElement | null;
    if (target?.closest('.pane-intelligence')) return;

    this.closePaneIntelligenceMenu(false);
  }

  get layoutColumns(): string {
    const left = this.clampPanePercent(this.leftPanePercent);
    return `minmax(0, ${left}fr) 5px minmax(0, ${100 - left}fr)`;
  }

  get shortcutHelpItems(): readonly {
    id: string;
    shortcut: string;
    description: string;
    category: string;
  }[] {
    return this.input.getBindingSnapshot()
      .filter(binding => binding.trigger.device === 'keyboard')
      .map(binding => ({
        id: binding.id,
        shortcut: this.formatKeyboardBinding(binding),
        description: binding.description,
        category: binding.category ?? 'General',
      }))
      .sort((left, right) =>
        left.category.localeCompare(right.category) ||
        left.description.localeCompare(right.description) ||
        left.shortcut.localeCompare(right.shortcut),
      );
  }

  get recentSyncRuns(): readonly SyncHistoryEntry[] {
    return this.operationHistory.recentSyncs(5);
  }

  get activeWorkspace() { return this.workspaceService.getActiveWorkspace(); }

  get activePane(): PaneHostComponent | undefined {
    return this.activePaneId === 'left' ? this.leftPane : this.rightPane;
  }

  get otherPane(): PaneHostComponent | undefined {
    return this.activePaneId === 'left' ? this.rightPane : this.leftPane;
  }

  pluginSelectionPaths(): string[] {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return [];
    return pane.selection.selectedEntries
      .filter(entry => entry.path && entry.path !== '__parent__')
      .map(entry => entry.path);
  }

  pluginToolbarActionsFor(
    pane: PaneHostComponent | undefined,
  ): readonly PresentedCommand[] {
    if (!pane?.contextState.isDirectoryTab) return [];

    const context: ToolbarContext = {
      selectedPaths: pane.selection.selectedEntries.map(entry => entry.path),
      currentDirectory: pane.contextState.path,
    };

    return adaptPluginToolbarActions(
      this.pluginManager.resolveToolbarActions(context),
      context,
      this.input,
    );
  }

  private pluginContextMenuItems(
    pane: PaneHostComponent | undefined,
    targetPath: string,
  ): ContextMenuItem[] {
    if (!pane?.contextState.isDirectoryTab) return [];

    const context: MenuContext = {
      targetPath,
      selectedPaths: pane.selection.selectedEntries.map(entry => entry.path),
      currentDirectory: pane.contextState.path,
    };

    const items: ContextMenuItem[] = [];

    for (const contribution of this.pluginManager.resolveContextMenuItems(context)) {
      const command = presentPluginCommand(contribution, context, this.input, {
        source: 'context-menu',
      });
      if (!command.enabled) continue;

      // The menu token is intentionally not the application command ID.
      // It binds the exact PresentedCommand closure created for this right-click,
      // preserving targetPath/currentDirectory/selection from the scored context.
      const menuCommandId = `reassembly.context.plugin.${++this.contextMenuActionSequence}`;
      this.contextMenuActions.set(menuCommandId, () => command.execute());

      items.push({
        commandId: menuCommandId,
        label: command.label,
        shortcut: command.shortcut,
        enabled: true,
      });
    }

    return items;
  }

  private async refreshCommandPaletteCommands(): Promise<void> {
    const pluginSnapshots = await this.pluginLifecycle.sync().catch(() => []);

    this.commandPaletteCommands = this.input
      .getCommandCatalog()
      .filter(command => command.registered)
      .map(command => ({
        id: command.id,
        title: command.title,
        category: command.category ?? 'General',
        shortcut: command.preferredBinding
          ? this.formatKeyboardBinding(command.preferredBinding)
          : undefined,
        execute: () => {
          this.input.dispatchCommandById(command.id, {
            source: 'palette',
            context: {
              paneId: this.activePaneId,
              path: this.activePane?.contextState.path ?? '',
              selectedPaths: this.pluginSelectionPaths(),
            },
          });
        },
        isEnabled: () => !!this.input.getCommandState(command.id)?.enabled,
      }));

    const registered = new Set(
      this.commandPaletteCommands.map(command => command.id),
    );

    for (const snapshot of pluginSnapshots) {
      const manifest = snapshot.plugin.discovered.manifest;

      for (const commandId of manifest.contributes?.commands ?? []) {
        if (!commandId || registered.has(commandId)) continue;

        this.commandPaletteCommands.push({
          id: commandId,
          title: commandId,
          category: `Plugin: ${manifest.name || manifest.id}`,
          execute: async () => {
            await this.pluginLifecycle.activateByEvent(`onCommand:${commandId}`);
            this.input.dispatchCommandById(commandId, {
              source: 'palette',
              context: {
                paneId: this.activePaneId,
                path: this.activePane?.contextState.path ?? '',
                selectedPaths: this.pluginSelectionPaths(),
              },
            });
          },
          isEnabled: () =>
            snapshot.plugin.readyToActivate &&
            !snapshot.plugin.state.quarantined,
        });

        registered.add(commandId);
      }
    }

    this.commandPaletteCommands.push(
      ...this.pluginSurfacePaletteCommands(),
    );

    this.cdr.markForCheck();
  }

  openSettings(tab?: 'plugins'): void {
    void this.settings.openSettings(tab);
  }

  async openPluginSurface(surfaceId: string): Promise<void> {
    const id = surfaceId.trim();
    if (!id) return;

    try {
      await this.pluginLifecycle.activateByEvent(`onSurface:${id}`);
    } catch {
      // Surface resolution remains authoritative after activation.
    }

    const contribution = this.surfaceRegistry
      .list()
      .find(surface => surface.id === id);
    if (!contribution) return;

    await this.activePane?.viewerCommands?.openSurfaceTab(
      contribution.id,
      contribution.id,
    );
  }

  private pluginSurfacePaletteCommands(): PaletteCommand[] {
    const builtins = new Set([
      'dir',
      'viewer',
      'search-results',
      'size-manager',
      'duplicates',
    ]);

    return this.surfaceRegistry
      .list()
      .filter(surface => !builtins.has(surface.id))
      .map(surface => ({
        id: `reassembly.pluginSurface.${surface.id}`,
        title: `Open ${surface.id}`,
        category: 'Plugin Surfaces',
        execute: () => this.openPluginSurface(surface.id),
        isEnabled: () => !!this.activePane,
      }));
  }

  private stopDriveLifecycle: (() => void) | null = null;

  ngOnInit(): void {
    this.builtinWorkspaceSurfaceActions.register();
    this.restoreWorkspaceLayout();
    this.syncFilesWorkspacePresentation();
    queueMicrotask(() => {
      const workspace = this.workspaceService.getActiveWorkspace();
      if (workspace) this.restorePaneIntelligenceForWorkspace(workspace.id);
    });

    this.stopDriveLifecycle = this.driveLifecycle.start(() => {
      void this.handleDriveTopologyChange();
    });

    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'pane',
      priority: 100,
      parentId: 'global',
      description: 'Reassembly dual-pane file manager',
    });

    this.registerCommands();
    for (const binding of BINDINGS) this.input.registerBinding(binding);
    this.input.activateContext(CONTEXT_ID);
  }

  ngOnDestroy(): void {
    this.clearPaneIntelligenceTypeahead();
    this.stopDriveLifecycle?.();
    this.stopDriveLifecycle = null;
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const commandId of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(commandId, OWNER_ID);
    }
  }

  get paneLocationRelation(): ReturnType<typeof analyzePaneLocations>['relation'] {
    return analyzePaneLocations(this.leftPane?.contextState.path ?? '', this.rightPane?.contextState.path ?? '').relation;
  }

  get paneRelationLabel(): string {
    const result = analyzePaneLocations(this.leftPane?.contextState.path ?? '', this.rightPane?.contextState.path ?? '');
    if (result.relation === 'same') return 'Same location';
    if (result.relation === 'left-inside-right') return `Left is ${result.depthDelta} level${result.depthDelta === 1 ? '' : 's'} deeper`;
    if (result.relation === 'right-inside-left') return `Right is ${result.depthDelta} level${result.depthDelta === 1 ? '' : 's'} deeper`;
    return 'Independent locations';
  }

  get paneRelationTitle(): string {
    return `Left: ${this.leftPane?.contextState.path || 'вЂ”'}\nRight: ${this.rightPane?.contextState.path || 'вЂ”'}`;
  }

  toggleMirrorPresentation(): void {
    this.mirrorPresentation = !this.mirrorPresentation;
    if (!this.mirrorPresentation) this.mirrorFilter = false;

    if (this.mirrorPresentation && this.activePane) {
      const presentation = this.activePane.presentation?.panePresentation();
      if (presentation) {
        this.onPanePresentationChange(this.activePaneId, presentation);
      }
    }

    this.linkStatus = this.mirrorPresentation ? 'presentation mirrored' : '';
    this.persistPaneIntelligence();
    this.cdr.markForCheck();
  }

  toggleMirrorFilter(): void {
    if (!this.mirrorPresentation) return;
    this.mirrorFilter = !this.mirrorFilter;

    if (this.mirrorFilter && this.activePane) {
      const presentation = this.activePane.presentation?.panePresentation();
      if (presentation) {
        this.onPanePresentationChange(this.activePaneId, presentation);
      }
    }

    this.linkStatus = this.mirrorFilter ? 'filter mirrored' : 'filter independent';
    this.persistPaneIntelligence();
    this.cdr.markForCheck();
  }

  onPanePresentationChange(
    paneId: PaneId,
    presentation: PanePresentation,
  ): void {
    this.invalidatePaneComparison('pane presentation changed');
    if (!this.mirrorPresentation) return;

    const target = paneId === 'left' ? this.rightPane : this.leftPane;
    if (!target?.contextState.isDirectoryTab) return;

    target.presentation?.applyMirroredPresentation(presentation, {
      filter: this.mirrorFilter,
    });
    this.linkStatus = this.mirrorFilter
      ? 'presentation + filter aligned'
      : 'presentation aligned';
    this.cdr.markForCheck();
  }

  toggleLinkedNavigation(): void {
    if (!this.linkedNavigation) {
      void this.ensurePaneIntelligenceHealth('enable link').then(ok => {
        if (!ok) return;
      });
    }

    this.linkedNavigation = !this.linkedNavigation;
    this.linkStatus = '';

    if (this.linkedNavigation) {
      this.linkedRoots = { left:this.leftPane?.contextState.path ?? '', right:this.rightPane?.contextState.path ?? '' };
      this.linkStatus = 'roots linked';
    } else {
      this.linkedRoots = null;
    }
    this.persistPaneIntelligence();
    this.cdr.markForCheck();
  }

  togglePaneIntelligenceMenu(event?: Event): void {
    event?.stopPropagation();

    if (this.paneIntelligenceMenuOpen) {
      this.closePaneIntelligenceMenu(true);
      return;
    }

    this.paneIntelligenceMenuOpen = true;
    this.clearPaneIntelligenceTypeahead();
    this.cdr.detectChanges();

    queueMicrotask(() => {
      this.focusPaneIntelligenceItem(0, 1);
    });
  }

  closePaneIntelligenceMenu(restoreFocus = false): void {
    if (!this.paneIntelligenceMenuOpen) return;

    this.paneIntelligenceMenuOpen = false;
    this.clearPaneIntelligenceTypeahead();
    this.cdr.markForCheck();

    if (restoreFocus) {
      queueMicrotask(() => {
        this.paneIntelligenceMore?.nativeElement.focus({ preventScroll: true });
      });
    }
  }

  onPaneIntelligenceMenuKeyDown(event: KeyboardEvent): void {
    const items = this.paneIntelligenceItems();
    if (!items.length) return;

    const active = document.activeElement as HTMLElement | null;
    const current = Math.max(0, items.indexOf(active as HTMLButtonElement));

    switch (event.key) {
      case 'Escape':
        event.preventDefault();
        event.stopPropagation();
        this.closePaneIntelligenceMenu(true);
        return;

      case 'ArrowDown':
        event.preventDefault();
        this.focusPaneIntelligenceItem(current + 1, 1);
        return;

      case 'ArrowUp':
        event.preventDefault();
        this.focusPaneIntelligenceItem(current - 1, -1);
        return;

      case 'Home':
        event.preventDefault();
        this.focusPaneIntelligenceItem(0, 1);
        return;

      case 'End':
        event.preventDefault();
        this.focusPaneIntelligenceItem(items.length - 1, -1);
        return;

      case 'Enter':
      case ' ':
        if (active?.matches('[data-pane-intelligence-item]')) {
          event.preventDefault();
          (active as HTMLButtonElement).click();
        }
        return;
    }

    if (
      event.key.length === 1 &&
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey
    ) {
      const key = event.key.toLocaleLowerCase();
      this.paneIntelligenceTypeahead += key;

      if (this.paneIntelligenceTypeaheadTimer !== null) {
        clearTimeout(this.paneIntelligenceTypeaheadTimer);
      }
      this.paneIntelligenceTypeaheadTimer = setTimeout(
        () => this.clearPaneIntelligenceTypeahead(),
        700,
      );

      const enabled = items.filter(item => !item.disabled);
      const match = enabled.find(item =>
        (item.textContent ?? '').trim().toLocaleLowerCase()
          .startsWith(this.paneIntelligenceTypeahead),
      );

      if (match) {
        event.preventDefault();
        match.focus({ preventScroll: true });
      }
    }
  }

  private paneIntelligenceItems(): HTMLButtonElement[] {
    const root = this.paneIntelligenceMenu?.nativeElement;
    if (!root) return [];

    return Array.from(
      root.querySelectorAll<HTMLButtonElement>(
        'button[data-pane-intelligence-item]',
      ),
    );
  }

  private focusPaneIntelligenceItem(
    requestedIndex: number,
    direction: 1 | -1,
  ): void {
    const items = this.paneIntelligenceItems();
    if (!items.length) return;

    let index =
      ((requestedIndex % items.length) + items.length) % items.length;

    for (let attempts = 0; attempts < items.length; attempts += 1) {
      const item = items[index];
      if (!item.disabled) {
        item.focus({ preventScroll: true });
        return;
      }
      index =
        ((index + direction) % items.length + items.length) % items.length;
    }
  }

  private clearPaneIntelligenceTypeahead(): void {
    this.paneIntelligenceTypeahead = '';

    if (this.paneIntelligenceTypeaheadTimer !== null) {
      clearTimeout(this.paneIntelligenceTypeaheadTimer);
      this.paneIntelligenceTypeaheadTimer = null;
    }
  }

  async runPaneIntelligenceAction(
    action:
      | 'match'
      | 'swapRoots'
      | 'peer'
      | 'copyPeer'
      | 'movePeer'
      | 'prevDiff'
      | 'nextDiff'
      | 'filter'
      | 'savePreset'
      | 'applyPreset'
      | 'deletePreset',
  ): Promise<void> {
    this.closePaneIntelligenceMenu(false);

    switch (action) {
      case 'match':
        await this.matchOtherPaneToActive();
        break;
      case 'swapRoots':
        this.swapLinkedRoots();
        break;
      case 'peer':
        this.openCurrentCounterpart();
        break;
      case 'copyPeer':
        await this.transferCurrentToPeer('copy');
        break;
      case 'movePeer':
        await this.transferCurrentToPeer('move');
        break;
      case 'prevDiff':
        this.navigateDifference(-1);
        break;
      case 'nextDiff':
        this.navigateDifference(1);
        break;
      case 'filter':
        this.toggleMirrorFilter();
        break;
      case 'savePreset':
        await this.saveCurrentPanePreset();
        break;
      case 'applyPreset':
        await this.chooseAndApplyPanePreset();
        break;
      case 'deletePreset':
        await this.deleteSelectedPanePreset();
        break;
    }
  }

  get panePairPresets(): readonly PanePairPreset[] {
    return this.paneIntelligence.listPresets();
  }

  async onPanePresetSelected(id: string): Promise<void> {
    this.selectedPanePresetId = id;
    if (!id) {
      this.cdr.markForCheck();
      return;
    }

    const preset = this.paneIntelligence.preset(id);
    if (!preset) {
      this.selectedPanePresetId = '';
      this.cdr.markForCheck();
      return;
    }

    await this.applyPanePreset(preset);
  }

  async chooseAndApplyPanePreset(): Promise<void> {
    const presets = this.panePairPresets;
    if (!presets.length) return;

    const choice = (await this.confirmDialog.prompt({
      title: 'Apply Pane Pair Preset',
      message: [
        'Enter preset number or exact name:',
        ...presets.map((preset, index) => `${index + 1}. ${preset.name}`),
      ].join('\n'),
      confirmText: 'Apply',
      defaultValue: this.selectedPanePresetId
        ? this.paneIntelligence.preset(this.selectedPanePresetId)?.name ?? ''
        : '',
      historyKey: 'reassembly.panePairPreset.apply',
    }))?.trim();

    if (!choice) return;

    const numeric = Number(choice);
    const preset =
      Number.isInteger(numeric) && numeric >= 1 && numeric <= presets.length
        ? presets[numeric - 1]
        : presets.find(item =>
            item.name.localeCompare(choice, undefined, {
              sensitivity: 'base',
            }) === 0
          );

    if (!preset) {
      await this.confirmDialog.alert({
        title: 'Preset Not Found',
        message: `No pane pair preset matches "${choice}".`,
      });
      return;
    }

    this.selectedPanePresetId = preset.id;
    await this.applyPanePreset(preset);
  }

  async saveCurrentPanePreset(): Promise<void> {
    const left = this.leftPane?.contextState.path ?? '';
    const right = this.rightPane?.contextState.path ?? '';
    if (!left || !right) return;

    const suggested =
      this.selectedPanePresetId
        ? this.paneIntelligence.preset(this.selectedPanePresetId)?.name ?? ''
        : '';

    const name = (await this.confirmDialog.prompt({
      title: 'Save Pane Pair Preset',
      message: 'Name this left/right pane relationship.',
      confirmText: 'Save',
      defaultValue: suggested,
      historyKey: 'reassembly.panePairPreset.name',
    }))?.trim();

    if (!name) return;

    const preset = this.paneIntelligence.savePreset({
      id: this.selectedPanePresetId || undefined,
      name,
      roots: { left, right },
      modes: this.currentPaneIntelligenceModes(),
    });

    this.selectedPanePresetId = preset.id;
    this.linkStatus = `preset saved: ${preset.name}`;
    this.cdr.markForCheck();
  }

  async deleteSelectedPanePreset(): Promise<void> {
    if (!this.selectedPanePresetId) return;

    const preset = this.paneIntelligence.preset(this.selectedPanePresetId);
    if (!preset) {
      this.selectedPanePresetId = '';
      return;
    }

    const confirmed = await this.confirmDialog.confirm({
      title: 'Delete Pane Pair Preset',
      message: `Delete preset "${preset.name}"?`,
      confirmText: 'Delete',
    });
    if (!confirmed) return;

    this.paneIntelligence.deletePreset(preset.id);
    this.selectedPanePresetId = '';
    this.linkStatus = `preset deleted: ${preset.name}`;
    this.cdr.markForCheck();
  }

  private async applyPanePreset(
    preset: PanePairPreset,
  ): Promise<void> {
    const leftPane = this.leftPane;
    const rightPane = this.rightPane;
    if (!leftPane?.contextState.isDirectoryTab || !rightPane?.contextState.isDirectoryTab) return;

    const [leftExists, rightExists] = await Promise.all([
      leftPane.navigation?.canNavigateToLocation(preset.roots.left),
      rightPane.navigation?.canNavigateToLocation(preset.roots.right),
    ]);

    if (!leftExists || !rightExists) {
      this.linkStatus =
        !leftExists && !rightExists
          ? 'preset roots unavailable'
          : !leftExists
            ? `preset left unavailable: ${preset.roots.left}`
            : `preset right unavailable: ${preset.roots.right}`;
      this.cdr.markForCheck();
      return;
    }

    this.restoringPaneIntelligence = true;
    try {
      const [leftLoaded, rightLoaded] = await Promise.all([
        leftPane.navigation?.navigateLinkedLocation(preset.roots.left, {
          history: 'record',
          preserveCursor: false,
        }),
        rightPane.navigation?.navigateLinkedLocation(preset.roots.right, {
          history: 'record',
          preserveCursor: false,
        }),
      ]);

      if (!leftLoaded || !rightLoaded) {
        this.linkStatus = 'preset navigation failed';
        return;
      }

      this.invalidatePaneComparison('preset changed pane relationship');
      this.linkedRoots = {
        left: preset.roots.left,
        right: preset.roots.right,
      };
      this.applyPaneIntelligenceModes(preset.modes);
      this.selectedPanePresetId = preset.id;
      this.linkStatus = `preset: ${preset.name}`;
    } finally {
      this.restoringPaneIntelligence = false;
      this.persistPaneIntelligence();
      this.cdr.markForCheck();
    }
  }

  private currentPaneIntelligenceModes() {
    return {
      linkedNavigation: this.linkedNavigation,
      mirrorPresentation: this.mirrorPresentation,
      mirrorFilter: this.mirrorFilter,
      syncScroll: this.syncScroll,
      followSelection: this.followSelection,
    };
  }

  private applyPaneIntelligenceModes(
    modes: {
      linkedNavigation: boolean;
      mirrorPresentation: boolean;
      mirrorFilter: boolean;
      syncScroll: boolean;
      followSelection: boolean;
    },
  ): void {
    this.linkedNavigation = modes.linkedNavigation;
    this.mirrorPresentation = modes.mirrorPresentation;
    this.mirrorFilter = modes.mirrorPresentation && modes.mirrorFilter;
    this.syncScroll = modes.syncScroll;
    this.followSelection = modes.followSelection;
  }

  private persistPaneIntelligence(): void {
    if (this.restoringPaneIntelligence) return;

    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) return;

    this.paneIntelligence.saveWorkspace(workspace.id, {
      ...this.currentPaneIntelligenceModes(),
      roots: this.linkedRoots
        ? {
            left: this.linkedRoots.left,
            right: this.linkedRoots.right,
          }
        : null,
    });
  }

  private restorePaneIntelligenceForWorkspace(workspaceId: string): void {
    if (!workspaceId) return;

    const state = this.paneIntelligence.loadWorkspace(workspaceId);

    this.restoringPaneIntelligence = true;
    try {
      this.applyPaneIntelligenceModes(state);
      this.linkedRoots = state.roots
        ? { left: state.roots.left, right: state.roots.right }
        : null;

      // If Link was persisted before roots existed, establish the current
      // locations as a safe pair rather than restoring a broken link.
      if (
        this.linkedNavigation &&
        !this.linkedRoots &&
        this.leftPane?.contextState.path &&
        this.rightPane?.contextState.path
      ) {
        this.linkedRoots = {
          left: this.leftPane.contextState.path,
          right: this.rightPane.contextState.path,
        };
      }

      this.linkStatus =
        this.linkedNavigation ||
        this.mirrorPresentation ||
        this.syncScroll ||
        this.followSelection
          ? 'pane intelligence restored'
          : '';
    } finally {
      this.restoringPaneIntelligence = false;
      this.cdr.markForCheck();
    }
  }

  get canMatchOtherPane(): boolean {
    return !!this.activePane?.contextState.isDirectoryTab &&
      !!this.otherPane?.contextState.isDirectoryTab &&
      !!this.activePane.contextState.path;
  }

  get canSwapLinkedRoots(): boolean {
    return !!this.linkedRoots?.left && !!this.linkedRoots?.right;
  }

  get currentCorrespondence(): {
    name: string;
    status: string;
    activePath: string;
    peerPath: string | null;
    activeEntry: FileInfo;
    peerEntry: FileInfo | null;
  } | null {
    const source = this.activePane;
    const peer = this.otherPane;
    const cursor = source?.selection?.cursorEntry;

    if (!source || !peer || !cursor || cursor.path === '__parent__') return null;

    const item = this.paneComparisonResult.items.find(candidate => {
      const activeEntry = this.activePaneId === 'left'
        ? candidate.left
        : candidate.right;

      return activeEntry?.path === cursor.path ||
        activeEntry?.name.toLocaleLowerCase() === cursor.name.toLocaleLowerCase();
    });

    if (!item) {
      const peerEntry = peer.contextState.entries.find(candidate =>
        candidate.path !== '__parent__' &&
        candidate.name.toLocaleLowerCase() === cursor.name.toLocaleLowerCase()
      ) ?? null;

      return {
        name: cursor.name,
        status: peerEntry ? 'unverified-peer' : 'only-here',
        activePath: cursor.path,
        peerPath: peerEntry?.path ?? null,
        activeEntry: cursor,
        peerEntry,
      };
    }

    const peerEntry = this.activePaneId === 'left'
      ? item.right
      : item.left;

    return {
      name: item.name,
      status: item.status,
      activePath: cursor.path,
      peerPath: peerEntry?.path ?? null,
      activeEntry: cursor,
      peerEntry,
    };
  }

  get canOpenCurrentCounterpart(): boolean {
    return !!this.currentCorrespondence?.peerEntry;
  }

  get canCopyCurrentToPeer(): boolean {
    return this.canTransferCurrentToPeer('copy');
  }

  get canMoveCurrentToPeer(): boolean {
    return this.canTransferCurrentToPeer('move');
  }

  async matchOtherPaneToActive(): Promise<void> {
    const source = this.activePane;
    const target = this.otherPane;

    if (!source?.contextState.isDirectoryTab || !target?.contextState.isDirectoryTab || !source.contextState.path) {
      return;
    }

    if (source.contextState.path === target.contextState.path) {
      this.linkStatus = 'already matched';
      this.cdr.markForCheck();
      return;
    }

    if (!(await target.navigation?.canNavigateToLocation(source.contextState.path))) {
      this.linkStatus = `cannot match: ${source.contextState.path}`;
      this.cdr.markForCheck();
      return;
    }

    const loaded = await target.navigation?.navigateLinkedLocation(source.contextState.path, {
      history: 'record',
      preserveCursor: false,
    });

    this.linkStatus = loaded ? 'other pane matched' : 'match failed';
    if (loaded) this.invalidatePaneComparison('locations changed');
    this.cdr.markForCheck();
  }

  swapLinkedRoots(): void {
    if (!this.linkedRoots) return;

    this.linkedRoots = {
      left: this.linkedRoots.right,
      right: this.linkedRoots.left,
    };
    this.linkStatus = 'linked roots swapped';
    this.persistPaneIntelligence();
    this.cdr.markForCheck();
  }

  openCurrentCounterpart(): void {
    const correspondence = this.currentCorrespondence;
    const target = this.otherPane;
    if (!correspondence?.peerEntry || !target) return;

    const targetPaneId: PaneId =
      this.activePaneId === 'left' ? 'right' : 'left';

    target.selection?.selectComparisonItem(correspondence.peerEntry.path);
    this.activate(targetPaneId, true);
    void target.viewerCommands?.openEntry(correspondence.peerEntry);

    this.linkStatus = `opened peer: ${correspondence.name}`;
    this.cdr.markForCheck();
  }

  async transferCurrentToPeer(operation: 'copy' | 'move'): Promise<void> {
    const sourcePane = this.activePane;
    const destinationPane = this.otherPane;
    const source = sourcePane?.selection?.cursorEntry;

    if (
      !sourcePane?.contextState.isDirectoryTab ||
      !destinationPane?.contextState.isDirectoryTab ||
      !source ||
      source.path === '__parent__'
    ) {
      return;
    }

    const decision = canTransferBetween(
      operation,
      sourcePane.contextState.sourceCapabilities,
      destinationPane.contextState.sourceCapabilities,
    );

    if (!decision.allowed) {
      await this.confirmDialog.alert(
        transferBlockedMessage(operation, decision),
      );
      return;
    }

    const destination = await this.api.joinPaths([
      destinationPane.contextState.path,
      source.name,
    ]);

    if (destination === source.path) {
      this.linkStatus = 'same resource';
      this.cdr.markForCheck();
      return;
    }

    if (operation === 'move') {
      const confirmed = await this.confirmDialog.confirm({
        title: 'Move to Other Pane',
        message: `Move "${source.name}" to ${destinationPane.contextState.path}?`,
        confirmText: 'Move',
      });
      if (!confirmed) return;
    }

    try {
      const request = [{
        source: source.path,
        destination,
        exactDestination: true,
      }];

      const result = operation === 'copy'
        ? await this.api.batchCopy(request, true)
        : await this.api.batchMove(request, true);

      this.lastTransfer =
        `${operation} counterpart: ${result.succeeded}/${result.total}` +
        (result.skipped > 0 ? ` · ${result.skipped} skipped` : '') +
        (result.failed > 0 ? ` · ${result.failed} failed` : '');

      const affected = result.affectedPaths ?? [];
      if (
        result.failed === 0 &&
        result.skipped === 0 &&
        affected.length === 1
      ) {
        const item = affected[0];

        this.operationHistory.record({
          label: `${operation === 'copy' ? 'Copy' : 'Move'} ${source.name} to peer pane`,
          undo: async () => {
            if (operation === 'copy') {
              const deletion = await this.api.batchDelete(
                [item.destination],
                true,
                false,
              );
              if (deletion.failed > 0 || deletion.skipped > 0) {
                throw new Error('Undo counterpart copy failed.');
              }
              return;
            }

            const back = await this.api.batchMove([{
              source: item.destination,
              destination: item.source,
              exactDestination: true,
            }], true);

            if (back.failed > 0 || back.skipped > 0) {
              throw new Error('Undo counterpart move failed.');
            }
          },
          redo: async () => {
            const forward = [{
              source: item.source,
              destination: item.destination,
              exactDestination: true,
            }];

            const redone = operation === 'copy'
              ? await this.api.batchCopy(forward, true)
              : await this.api.batchMove(forward, true);

            if (redone.failed > 0 || redone.skipped > 0) {
              throw new Error(`Redo counterpart ${operation} failed.`);
            }
          },
        });
      }

      await Promise.all([
        destinationPane.navigation?.refresh(),
        operation === 'move'
          ? sourcePane.navigation?.refresh()
          : Promise.resolve(),
      ]);

      this.invalidatePaneComparison('file operation changed pane contents');
      sourcePane.selection?.recoverVisibleCursor();
      destinationPane.selection?.recoverVisibleCursor();

      if (result.succeeded > 0) {
        destinationPane.selection?.selectComparisonItem(destination);
        this.linkStatus =
          `${operation === 'copy' ? 'copied' : 'moved'} peer: ${source.name}`;
      }

      if (result.failed > 0 || result.skipped > 0) {
        await this.confirmDialog.alert({
          title: `${operation === 'copy' ? 'Copy' : 'Move'} Completed with Issues`,
          message: [
            result.failed > 0
              ? `${result.failed} item${result.failed === 1 ? '' : 's'} failed.`
              : '',
            result.skipped > 0
              ? `${result.skipped} item${result.skipped === 1 ? '' : 's'} were skipped.`
              : '',
          ].filter(Boolean).join('\n'),
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: `${operation === 'copy' ? 'Copy' : 'Move'} Counterpart Failed`,
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      this.cdr.markForCheck();
    }
  }

  private canTransferCurrentToPeer(operation: 'copy' | 'move'): boolean {
    const sourcePane = this.activePane;
    const destinationPane = this.otherPane;
    const source = sourcePane?.selection?.cursorEntry;

    if (
      !sourcePane?.contextState.isDirectoryTab ||
      !destinationPane?.contextState.isDirectoryTab ||
      !source ||
      source.path === '__parent__'
    ) {
      return false;
    }

    return canTransferBetween(
      operation,
      sourcePane.contextState.sourceCapabilities,
      destinationPane.contextState.sourceCapabilities,
    ).allowed;
  }

  private currentPanePairSignature(): string {
    return [
      this.leftPane?.contextState.path ?? '',
      this.rightPane?.contextState.path ?? '',
      this.leftPane?.contextState.visibleEntries?.length ?? 0,
      this.rightPane?.contextState.visibleEntries?.length ?? 0,
    ].join('|');
  }

  private invalidatePaneComparison(reason = 'comparison stale'): void {
    this.differenceCursor = -1;
    this.comparisonSignature = '';
    this.linkStatus = reason;
    this.cdr.markForCheck();
  }

  private comparisonIsCurrent(): boolean {
    if (!this.paneComparisonResult) return false;
    if (!this.comparisonSignature) return false;
    return this.comparisonSignature === this.currentPanePairSignature();
  }

  private async ensurePaneIntelligenceHealth(
    reason: string,
  ): Promise<boolean> {
    if (this.recoveringPaneIntelligence) return false;

    const left = this.leftPane;
    const right = this.rightPane;
    if (!left || !right) return false;

    this.recoveringPaneIntelligence = true;
    const epoch = ++this.paneIntelligenceEpoch;

    try {
      const [leftAvailable, rightAvailable] = await Promise.all([
        left.navigation?.currentLocationAvailable(),
        right.navigation?.currentLocationAvailable(),
      ]);

      if (epoch !== this.paneIntelligenceEpoch) return false;

      if (!leftAvailable || !rightAvailable) {
        this.disableUnsafePaneCooperation({
          keepPresentation: true,
          message:
            !leftAvailable && !rightAvailable
              ? `${reason}: both pane locations unavailable`
              : !leftAvailable
                ? `${reason}: left pane location unavailable`
                : `${reason}: right pane location unavailable`,
        });
        return false;
      }

      if (this.linkedNavigation && this.linkedRoots) {
        const [leftRootAvailable, rightRootAvailable] = await Promise.all([
          left.navigation?.canNavigateToLocation(this.linkedRoots.left),
          right.navigation?.canNavigateToLocation(this.linkedRoots.right),
        ]);

        if (epoch !== this.paneIntelligenceEpoch) return false;

        if (!leftRootAvailable || !rightRootAvailable) {
          this.linkedNavigation = false;
          this.linkedRoots = null;
          this.linkStatus =
            !leftRootAvailable && !rightRootAvailable
              ? 'link disabled: both linked roots unavailable'
              : !leftRootAvailable
                ? 'link disabled: left linked root unavailable'
                : 'link disabled: right linked root unavailable';
          this.persistPaneIntelligence();
        }
      }

      return true;
    } finally {
      this.recoveringPaneIntelligence = false;
      this.cdr.markForCheck();
    }
  }

  private disableUnsafePaneCooperation(options: {
    keepPresentation: boolean;
    message: string;
  }): void {
    this.linkedNavigation = false;
    this.followSelection = false;
    this.syncScroll = false;
    this.linkedRoots = null;

    if (!options.keepPresentation) {
      this.mirrorPresentation = false;
      this.mirrorFilter = false;
    }

    this.differenceCursor = -1;
    this.comparisonSignature = '';
    this.linkStatus = options.message;
    this.persistPaneIntelligence();
    this.cdr.markForCheck();
  }

  private markComparisonCurrent(): void {
    this.comparisonSignature = this.currentPanePairSignature();
    this.differenceCursor = -1;
  }


  get differenceCount(): number {
    return this.comparisonIsCurrent()
      ? this.comparisonDifferences.length
      : 0;
  }

  get differenceIndexLabel(): string {
    if (!this.differenceCount) return '';
    return `${this.differenceCursor >= 0 ? this.differenceCursor + 1 : 0}/${this.differenceCount}`;
  }

  toggleSyncScroll(): void {
    this.syncScroll = !this.syncScroll;
    this.linkStatus = this.syncScroll ? 'scroll sync on' : '';
    this.persistPaneIntelligence();
    this.cdr.markForCheck();

    if (this.syncScroll && this.activePane) {
      const snapshot = this.activePane.viewport?.viewportSnapshot();
      if (snapshot) {
        this.onPaneViewportChange(this.activePaneId, snapshot);
      }
    }
  }

  onPaneViewportChange(
    paneId: PaneId,
    snapshot: { topName: string | null; ratio: number },
  ): void {
    if (!this.syncScroll || this.scrollSyncGuard) return;

    void this.ensurePaneIntelligenceHealth('scroll sync');
    const target = paneId === 'left' ? this.rightPane : this.leftPane;
    if (!target?.contextState.isDirectoryTab) return;

    this.scrollSyncGuard = true;
    try {
      target.viewport?.syncViewportToPeer(snapshot);
      this.linkStatus = snapshot.topName
        ? `aligned near: ${snapshot.topName}`
        : 'scroll ratio aligned';
    } finally {
      queueMicrotask(() => {
        this.scrollSyncGuard = false;
        this.cdr.markForCheck();
      });
    }
  }

  navigateDifference(delta: -1 | 1): void {
    if (!this.comparisonIsCurrent()) {
      this.invalidatePaneComparison('comparison changed; run Compare again');
      return;
    }

    const differences = this.comparisonDifferences;
    if (!differences.length) return;

    this.differenceCursor = this.differenceCursor < 0
      ? (delta > 0 ? 0 : differences.length - 1)
      : (this.differenceCursor + delta + differences.length) % differences.length;

    const target = differences[this.differenceCursor];
    if (target.leftPath) this.leftPane?.selection?.selectComparisonItem(target.leftPath);
    if (target.rightPath) this.rightPane?.selection?.selectComparisonItem(target.rightPath);
    this.linkStatus = `difference: ${target.name}`;
    this.cdr.markForCheck();
  }

  private get comparisonDifferences(): Array<{
    name: string;
    leftPath: string | null;
    rightPath: string | null;
  }> {
    const result = this.paneComparisonResult;
    if (!result) return [];

    return result.items
      .filter(item => item.status !== 'same')
      .map(item => ({
        name: item.name ?? item.key,
        leftPath: item.left?.path ?? null,
        rightPath: item.right?.path ?? null,
      }));
  }


  toggleFollowSelection(): void {
    this.followSelection = !this.followSelection;
    this.linkStatus = this.followSelection ? 'selection follow on' : '';
    this.persistPaneIntelligence();
    this.cdr.markForCheck();

    const cursor = this.activePane?.selection.cursorEntry;
    if (this.followSelection && cursor) {
      this.onLinkedPaneCursorChange(this.activePaneId, cursor);
    }
  }

  async onLinkedPaneLocationChange(paneId: PaneId, path: string): Promise<void> {
    this.invalidatePaneComparison('locations changed');

    if (!(await this.ensurePaneIntelligenceHealth('linked navigation'))) {
      return;
    }
    if (!this.linkedNavigation || !this.linkedRoots || this.linkedNavigationGuard > 0) {
      this.cdr.markForCheck();
      return;
    }

    const otherId: PaneId = paneId === 'left' ? 'right' : 'left';
    const targetPane = otherId === 'left' ? this.leftPane : this.rightPane;
    if (!targetPane) return;

    const mirrored = mirrorPaneLocation(path, this.linkedRoots[paneId], this.linkedRoots[otherId]);
    if (!mirrored) {
      this.linkStatus = 'outside linked root';
      this.cdr.markForCheck();
      return;
    }

    if (mirrored === targetPane.contextState.path) {
      this.linkStatus = 'aligned';
      this.cdr.markForCheck();
      return;
    }

    let peer = mirrored;
    let exact = true;

    if (!(await targetPane.navigation?.canNavigateToLocation(peer))) {
      exact = false;
      const candidates = mirroredPeerCandidates(
        path,
        this.linkedRoots[paneId],
        this.linkedRoots[otherId],
      ).slice(1);

      peer = '';
      for (const candidate of candidates) {
        if (await targetPane.navigation?.canNavigateToLocation(candidate)) {
          peer = candidate;
          break;
        }
      }

      if (!peer) {
        this.linkStatus = `missing peer: ${this.leafName(mirrored) || mirrored}`;
        this.cdr.markForCheck();
        return;
      }
    }

    this.linkedNavigationGuard++;
    try {
      const loaded = await targetPane.navigation?.navigateLinkedLocation(peer, {
        history: 'record',
        preserveCursor: false,
      });
      this.linkStatus = loaded
        ? (exact ? 'aligned' : `nearest peer: ${this.leafName(peer) || peer}`)
        : 'peer navigation failed';
    } finally {
      this.linkedNavigationGuard = Math.max(0, this.linkedNavigationGuard - 1);
      this.cdr.markForCheck();
    }
  }

  onLinkedPaneCursorChange(paneId: PaneId, entry: FileInfo | null): void {
    if (!this.followSelection || !entry || this.linkedSelectionGuard) return;

    void this.ensurePaneIntelligenceHealth('follow selection');
    const targetPane = paneId === 'left' ? this.rightPane : this.leftPane;
    if (!targetPane) return;

    this.linkedSelectionGuard = true;
    try {
      const followed = targetPane.selection?.selectPeerByName(entry.name);
      this.linkStatus = followed ? `follow: ${entry.name}` : `no peer: ${entry.name}`;
    } finally {
      this.linkedSelectionGuard = false;
      this.cdr.markForCheck();
    }
  }


  activate(paneId: PaneId, focusContent = false): void {
    this.closePaneIntelligenceMenu();
    this.closeContextMenu();

    if (this.activePaneId !== paneId) {
      this.activePaneId = paneId;
      this.workspaceService.setActivePaneId(paneId);
      this.cdr.markForCheck();
    }

    if (focusContent) {
      queueMicrotask(() => this.activePane?.navigation?.focusContent());
    }
  }

  showShortcutReference(): void {
    const help = this.keyboardHelp;
    if (!help) {
      console.warn('[shortcuts] KeyboardHelpComponent is not mounted.');
      return;
    }
    help.show();
  }

  toggleShortcutHelp(): void {
    this.shortcutHelpVisible = !this.shortcutHelpVisible;
    this.cdr.markForCheck();
  }

  switchPane(): void {
    this.activate(
      this.activePaneId === 'left' ? 'right' : 'left',
      true,
    );
  }

  adjustSplit(delta: number): void {
    this.leftPanePercent = this.clampPanePercent(
      this.leftPanePercent + delta,
    );
    this.workspaceService.setSplitRatio(this.leftPanePercent);
    this.cdr.markForCheck();
  }

  openGlobalSearch(): void {
    const root = this.activePane?.contextState.isDirectoryTab
      ? this.activePane.contextState.path
      : this.otherPane?.contextState.isDirectoryTab
        ? this.otherPane.contextState.path
        : '';
    if (!root) return;
    this.globalSearchOverlay?.open(root);
  }

  openHistoryFor(paneId: PaneId, mode: 'pane' | 'all' | 'frequent' = 'pane'): void {
    this.activate(paneId);
    this.pathHistory?.open(mode);
  }

  openQuickAccessFor(paneId: PaneId, sectionId?: string): void {
    this.activate(paneId);
    void this.quickAccess?.open(sectionId);
  }

  async quickPathJump(): Promise<void> {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return;

    const path = (await this.confirmDialog.prompt({
      title: 'Go to Path',
      message: 'Enter a local, VFS, or remote location.',
      confirmText: 'Go',
      defaultValue: pane.contextState.path,
      historyKey: 'reassembly.path.jump',
    }))?.trim();

    if (!path) return;
    pane.navigation?.navigateTo(path);
  }

  quickViewActive(): void {
    const pane = this.activePane;
    const entry = pane?.selection.cursorEntry;
    if (!pane?.contextState.isDirectoryTab || !entry) return;
    if (entry.type === FileType.DIRECTORY) return;
    pane.viewerCommands?.openQuickView(entry);
  }

  async openActiveExternally(): Promise<void> {
    const pane = this.activePane;
    if (!pane) return;

    if (pane.tabState.activeTab?.kind === 'viewer') {
      await pane.viewerCommands?.openActiveViewerExternally();
      return;
    }

    const entry = pane.selection.cursorEntry;
    if (!pane.contextState.isDirectoryTab || !entry) return;
    if (entry.type === FileType.DIRECTORY) return;

    try {
      await this.api.openFile(entry.path);
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Open Externally Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async openActiveWithChooser(): Promise<void> {
    const pane = this.activePane;
    const entry =
      pane?.tabState.activeTab?.kind === 'viewer'
        ? pane.tabState.activeViewerTab
        : pane?.selection.cursorEntry;

    const path =
      pane?.tabState.activeTab?.kind === 'viewer'
        ? pane.tabState.activeViewerTab?.targetPath
        : pane?.selection.cursorEntry?.path;

    if (!pane || !path || pane.contextState.isInArchive) return;
    if (pane.tabState.activeTab?.kind !== 'viewer' &&
        pane.selection.cursorEntry?.type === FileType.DIRECTORY) return;

    try {
      const osInfo = await this.api.getOSInfo();
      if (String(osInfo?.os).toLowerCase() === 'windows') {
        await this.api.openFileWithChooser(path);
        return;
      }

      const application = (await this.confirmDialog.prompt({
        title: 'Open With',
        message:
          String(osInfo?.os).toLowerCase() === 'macos'
            ? 'Application name (for example: TextEdit, Preview, Visual Studio Code):'
            : 'Application executable or command name (for example: code, gedit, libreoffice):',
        confirmText: 'Open',
        defaultValue: '',
        historyKey: 'reassembly.openWith.application',
      }))?.trim();

      if (!application) return;
      await this.api.openFileWithApplication(path, application);
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Open With Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleDriveTopologyChange(): Promise<void> {
    await this.ensurePaneIntelligenceHealth('storage topology changed');
    await Promise.all([
      this.leftPane?.contextState.isDirectoryTab ? this.leftPane.navigation?.refresh() : Promise.resolve(),
      this.rightPane?.contextState.isDirectoryTab ? this.rightPane.navigation?.refresh() : Promise.resolve(),
    ]);
    this.lastTransfer = 'drives/roots changed';
    this.cdr.markForCheck();
  }


  openRemoteConnections(): void {
    this.remoteConnections?.open();
  }

  navigateFromRemoteConnections(path: string): void {
    const pane = this.activePane?.contextState.isDirectoryTab ? this.activePane :
      this.otherPane?.contextState.isDirectoryTab ? this.otherPane : null;
    if (!pane || !path) return;
    pane.navigation?.navigateTo(path);
    this.cdr.markForCheck();
  }

  async connectRemote(): Promise<void> {
    const mount = await this.remoteConnectDialog.openDialog();
    const path = mount?.publicPath?.trim();
    if (!path) return;
    this.navigateFromRemoteConnections(path);
  }

  openSearchResult(entry: FileInfo): void {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return;

    if (entry.type === FileType.DIRECTORY) {
      pane.navigation?.navigateTo(entry.path);
      return;
    }

    pane.viewerCommands?.openViewer(entry);
  }

  async openSearchResultsPanel(
    request: SearchResultsPanelRequest,
  ): Promise<void> {
    const pane = this.activePane?.contextState.isDirectoryTab
      ? this.activePane
      : this.otherPane?.contextState.isDirectoryTab
        ? this.otherPane
        : null;

    if (!pane || request.entries.length === 0) return;

    await pane.viewerCommands?.openResultsPanel(
      request.title,
      request.entries,
      request.query.startsWith('recent:')
        ? {
            origin: 'recent-files',
            query: request.query.slice('recent:'.length),
            kind: 'recent-files',
          }
        : {
            origin: 'global-search',
            rootPath: request.rootPath,
            query: request.query,
            kind: request.recursive
              ? 'recursive-search'
              : 'search',
          },
    );

    this.cdr.markForCheck();
  }

  async openDiscoveryPanelPath(path: string): Promise<void> {
    if (!/^results:\/\//i.test(path)) return;

    const pane = this.activePane?.contextState.isDirectoryTab
      ? this.activePane
      : this.otherPane?.contextState.isDirectoryTab
        ? this.otherPane
        : null;

    if (!pane) return;

    const loaded = await pane.navigation?.navigateLinkedLocation(path, {
      history: 'record',
      preserveCursor: false,
    });

    if (!loaded) {
      await this.confirmDialog.alert({
        title: 'Collection Unavailable',
        message: 'This results collection is no longer available.',
      });
    }

    this.globalSearchOverlay?.close();
    this.cdr.markForCheck();
  }

  openSearchWorkspace(
    request: SearchResultsPanelRequest,
  ): void {
    if (request.entries.length === 0) return;

    this.workspacePresentation = this.workspaceSurfaces.openToolWorkspace(
      request.title,
      surfaceLayout(
        workspaceSurface(
          `search:${Date.now()}`,
          'search-workspace',
          request.title,
          'result',
          {
            rootPath: request.rootPath,
            query: request.query,
            recursive: request.recursive,
            entries: request.entries.map(entry => ({ ...entry })),
            busy: false,
            error: '',
            refreshedAt: new Date().toLocaleTimeString(),
          },
          [
            { id: 'refresh', title: 'Refresh' },
            { id: 'open', title: 'Open', intent: 'primary', requiresSelection: true },
            { id: 'reveal', title: 'Reveal', requiresSelection: true },
            { id: 'copy-path', title: 'Copy Path', requiresSelection: true },
          ],
        ),
      ),
    );

    this.globalSearchOverlay?.close();
    this.cdr.markForCheck();
  }

  openCompareWorkspace(): void {
    if (!this.canComparePanes() || !this.leftPane || !this.rightPane) return;

    const result = this.paneComparisonResult;
    const rows = result.items.map(item => ({
      key: item.key,
      status: item.status,
      leftPath: item.left?.path ?? '',
      rightPath: item.right?.path ?? '',
    }));

    const summary = result.summary;
    this.workspacePresentation = this.workspaceSurfaces.openToolWorkspace(
      `Compare: ${this.leafName(this.leftPane.contextState.path)} в†” ${this.leafName(this.rightPane.contextState.path)}`,
      splitLayout(
        'vertical',
        surfaceLayout(
          workspaceSurface(
            `compare:${Date.now()}`,
            'compare-workspace',
            'Comparison',
            'primary',
            {
              leftPath: this.leftPane.contextState.path,
              rightPath: this.rightPane.contextState.path,
              recursive: this.comparisonRecursive,
              rows,
              busy: false,
              error: '',
              refreshedAt: new Date().toLocaleTimeString(),
            },
            [
              { id: 'refresh', title: 'Refresh' },
              { id: 'copy-left-right', title: 'Copy Missing в†’', intent: 'primary' },
              { id: 'copy-right-left', title: 'в†ђ Copy Missing', intent: 'primary' },
            ],
          ),
        ),
        surfaceLayout(
          workspaceSurface(
            `compare-summary:${Date.now()}`,
            'compare-summary',
            'Summary',
            'support',
            {
              items: [
                { label: 'Same', value: summary.same },
                { label: 'Different', value: summary.different },
                { label: 'Only left', value: summary.onlyLeft },
                { label: 'Only right', value: summary.onlyRight },
              ],
            },
          ),
        ),
        78,
      ),
    );

    this.cdr.markForCheck();
  }

  async openDetailsWorkspace(): Promise<void> {
    const pane = this.activePane;
    const entry = pane?.selection.cursorEntry;
    if (!pane || !entry) return;

    try {
      const info = pane.contextState.isInArchive
        ? entry
        : await this.api.getFileInfo(entry.path);

      const timestamp = (value: unknown): string => {
        const numeric = Number(value);
        if (!Number.isFinite(numeric) || numeric <= 0) return 'вЂ”';
        return new Date(
          numeric * (numeric < 10_000_000_000 ? 1000 : 1),
        ).toLocaleString();
      };

      const size =
        !pane.contextState.isInArchive && info.type === FileType.DIRECTORY
          ? await this.api.dirSizeRecursive(info.path)
          : Number(info.size) || 0;

      const type =
        info.type === FileType.DIRECTORY
          ? 'Directory'
          : info.type === FileType.ARCHIVE
            ? 'Archive'
            : info.type === FileType.SYMLINK
              ? 'Symbolic link'
              : 'File';

      this.workspacePresentation = this.workspaceSurfaces.openToolWorkspace(
        `Details: ${info.name}`,
        surfaceLayout(
          workspaceSurface(
            `details:${Date.now()}`,
            'details-workspace',
            info.name,
            'primary',
            {
              path: info.path,
              sourceKind: pane.contextState.activePanelSource.source.kind,
              inArchive: pane.contextState.isInArchive,
              busy: false,
              error: '',
              refreshedAt: new Date().toLocaleTimeString(),
              items: [
                { label: 'Name', value: info.name },
                { label: 'Type', value: type },
                { label: 'Path', value: info.path },
                { label: 'Size', value: `${size.toLocaleString()} bytes` },
                { label: 'Modified', value: timestamp(info.modified) },
                { label: 'Created', value: timestamp(info.created) },
                { label: 'Accessed', value: timestamp(info.accessed) },
                { label: 'Permissions', value: info.permissions || 'вЂ”' },
                { label: 'Symbolic link', value: info.isSymlink ? 'Yes' : 'No' },
                { label: 'Source', value: pane.contextState.activePanelSource.source.title },
              ],
            },
            pane.contextState.isInArchive
              ? [{ id: 'refresh', title: 'Refresh' }]
              : [
                  { id: 'refresh', title: 'Refresh' },
                  { id: 'rename', title: 'Rename', intent: 'primary' },
                  { id: 'modified-time', title: 'Modified Time' },
                  { id: 'attributes', title: 'Attributes' },
                  { id: 'reveal', title: 'Reveal' },
                ],
          ),
        ),
      );

      this.cdr.markForCheck();
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Open Details Workspace Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async handleWorkspaceSurfaceAction(event: {
    surface: WorkspaceSurface;
    action: string;
    payload?: unknown;
  }): Promise<void> {
    const workspace = this.workspacePresentation;
    if (!workspace || workspace.kind !== 'tool') return;

    if (event.action === 'refresh') {
      await this.refreshWorkspaceSurface(event.surface);
      return;
    }

    const handled = await this.workspaceSurfaceActions.dispatch(
      {
        workspace,
        surface: event.surface,
        action: event.action,
        payload: event.payload,
      },
      {
        refreshSurface: surface =>
          this.refreshWorkspaceSurface(surface),

        patchSurface: (surfaceId, patch) =>
          this.patchLiveSurface(surfaceId, patch),

        replaceSurfaceState: (surfaceId, state) => {
          const updated = this.workspaceSurfaces.replaceSurfaceState(
            workspace.workspaceId,
            surfaceId,
            state,
          );
          if (updated) {
            this.workspacePresentation = updated;
            this.cdr.markForCheck();
          }
        },

        findSurfaceByKind: surfaceKind =>
          this.findWorkspaceSurfaceByKind(surfaceKind),

        getActivePane: () => this.activePane,

        closeToolWorkspace: () => this.closeToolWorkspace(),

        markForCheck: () => this.cdr.markForCheck(),
      },
    );

    if (!handled) {
      this.patchLiveSurface(event.surface.id, {
        error: `No handler registered for action "${event.action}".`,
      });
    }
  }

  private async refreshWorkspaceSurface(
    surface: WorkspaceSurface,
  ): Promise<void> {
    switch (surface.surfaceId) {
      case 'search-workspace':
        await this.refreshSearchWorkspaceSurface(surface);
        return;

      case 'compare-workspace':
        await this.refreshCompareWorkspaceSurface(surface);
        return;

      case 'details-workspace':
        await this.refreshDetailsWorkspaceSurface(surface);
        return;
    }
  }


  private async refreshSearchWorkspaceSurface(
    surface: WorkspaceSurface,
  ): Promise<void> {
    const workspace = this.workspacePresentation;
    if (!workspace || workspace.kind !== 'tool') return;

    const rootPath = String(surface.state?.['rootPath'] ?? '');
    const query = String(surface.state?.['query'] ?? '').trim();
    const recursive = surface.state?.['recursive'] !== false;
    if (!rootPath || !query) return;

    this.patchLiveSurface(surface.id, {
      busy: true,
      error: '',
    });

    try {
      const result = await this.api.search(
        rootPath,
        query,
        recursive,
      );

      const entries = (result.matches ?? []).map(entry => ({
        ...entry,
      }));

      this.patchLiveSurface(surface.id, {
        entries,
        busy: false,
        error: '',
        refreshedAt: new Date().toLocaleTimeString(),
      });
    } catch (error) {
      this.patchLiveSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async refreshCompareWorkspaceSurface(
    surface: WorkspaceSurface,
  ): Promise<void> {
    const workspace = this.workspacePresentation;
    if (!workspace || workspace.kind !== 'tool') return;

    const leftPath = String(surface.state?.['leftPath'] ?? '');
    const rightPath = String(surface.state?.['rightPath'] ?? '');
    const recursive = surface.state?.['recursive'] === true;
    if (!leftPath || !rightPath) return;

    this.patchLiveSurface(surface.id, {
      busy: true,
      error: '',
    });

    try {
      let result: PaneComparisonResult;

      if (recursive) {
        const recursiveResult =
          await this.recursivePaneComparison.compare(
            this.api,
            leftPath,
            rightPath,
            { showHidden: this.showHidden },
          );
        result = recursiveResult.comparison;
      } else {
        const [left, right] = await Promise.all([
          this.api.listDirectory(leftPath, this.showHidden),
          this.api.listDirectory(rightPath, this.showHidden),
        ]);

        result = this.paneComparison.compare(
          left.entries ?? [],
          right.entries ?? [],
        );
      }

      const rows = result.items.map(item => ({
        key: item.key,
        status: item.status,
        leftPath: item.left?.path ?? '',
        rightPath: item.right?.path ?? '',
      }));

      this.patchLiveSurface(surface.id, {
        rows,
        busy: false,
        error: '',
        refreshedAt: new Date().toLocaleTimeString(),
      });

      const summarySurface =
        this.findWorkspaceSurfaceByKind('compare-summary');

      if (summarySurface) {
        const summary = result.summary;
        this.patchLiveSurface(summarySurface.id, {
          items: [
            { label: 'Same', value: summary.same },
            { label: 'Different', value: summary.different },
            { label: 'Only left', value: summary.onlyLeft },
            { label: 'Only right', value: summary.onlyRight },
          ],
          refreshedAt: new Date().toLocaleTimeString(),
        });
      }
    } catch (error) {
      this.patchLiveSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async refreshDetailsWorkspaceSurface(
    surface: WorkspaceSurface,
  ): Promise<void> {
    const workspace = this.workspacePresentation;
    if (!workspace || workspace.kind !== 'tool') return;

    const path = String(surface.state?.['path'] ?? '');
    const inArchive = surface.state?.['inArchive'] === true;
    if (!path) return;

    this.patchLiveSurface(surface.id, {
      busy: true,
      error: '',
    });

    try {
      if (inArchive) {
        throw new Error(
          'Archive-entry details are snapshot-based until archive stat support is exposed through PanelSource.',
        );
      }

      const info = await this.api.getFileInfo(path);
      const size =
        info.type === FileType.DIRECTORY
          ? await this.api.dirSizeRecursive(info.path)
          : Number(info.size) || 0;

      const timestamp = (value: unknown): string => {
        const numeric = Number(value);
        if (!Number.isFinite(numeric) || numeric <= 0) return 'вЂ”';
        return new Date(
          numeric * (numeric < 10_000_000_000 ? 1000 : 1),
        ).toLocaleString();
      };

      const type =
        info.type === FileType.DIRECTORY
          ? 'Directory'
          : info.type === FileType.ARCHIVE
            ? 'Archive'
            : info.type === FileType.SYMLINK
              ? 'Symbolic link'
              : 'File';

      this.patchLiveSurface(surface.id, {
        path: info.path,
        items: [
          { label: 'Name', value: info.name },
          { label: 'Type', value: type },
          { label: 'Path', value: info.path },
          { label: 'Size', value: `${size.toLocaleString()} bytes` },
          { label: 'Modified', value: timestamp(info.modified) },
          { label: 'Created', value: timestamp(info.created) },
          { label: 'Accessed', value: timestamp(info.accessed) },
          { label: 'Permissions', value: info.permissions || 'вЂ”' },
          { label: 'Symbolic link', value: info.isSymlink ? 'Yes' : 'No' },
          {
            label: 'Source',
            value: String(surface.state?.['sourceKind'] ?? 'filesystem'),
          },
        ],
        busy: false,
        error: '',
        refreshedAt: new Date().toLocaleTimeString(),
      });
    } catch (error) {
      this.patchLiveSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private patchLiveSurface(
    surfaceId: string,
    patch: Readonly<Record<string, unknown>>,
  ): void {
    const workspace = this.workspacePresentation;
    if (!workspace || workspace.kind !== 'tool') return;

    const updated = this.workspaceSurfaces.updateSurfaceState(
      workspace.workspaceId,
      surfaceId,
      patch,
    );

    if (updated) {
      this.workspacePresentation = updated;
      this.cdr.markForCheck();
    }
  }

  private findWorkspaceSurfaceByKind(
    surfaceKind: string,
  ): WorkspaceSurface | null {
    const workspace = this.workspacePresentation;
    if (!workspace || workspace.kind !== 'tool') return null;

    const visit = (layout: any): WorkspaceSurface | null => {
      if (!layout) return null;

      if (layout.kind === 'surface') {
        return layout.surface?.surfaceId === surfaceKind
          ? layout.surface
          : null;
      }

      if (layout.kind === 'split') {
        return visit(layout.first) ?? visit(layout.second);
      }

      if (layout.kind === 'stack') {
        return (
          layout.surfaces?.find(
            (surface: WorkspaceSurface) =>
              surface.surfaceId === surfaceKind,
          ) ?? null
        );
      }

      return null;
    };

    return visit(workspace.layout);
  }

  activateToolWorkspace(
    presentation: WorkspacePresentation,
  ): void {
    if (presentation.kind !== 'tool') return;

    this.workspacePresentation = presentation;
    this.closeContextMenu();
    this.cdr.markForCheck();
  }

  closeToolWorkspace(): void {
    const current = this.workspacePresentation;
    if (current?.kind === 'tool' && current.lifetime === 'transient') {
      this.workspaceSurfaces.closeTransient(current.workspaceId);
    }

    const files = this.workspaceService.getActiveWorkspace();
    if (files) {
      this.workspacePresentation =
        this.workspaceSurfaces.activateFilesWorkspace(files);
    } else {
      this.workspacePresentation = null;
    }

    queueMicrotask(() => this.activePane?.navigation?.focusContent());
    this.cdr.markForCheck();
  }

  async pinToolWorkspace(): Promise<void> {
    const current = this.workspacePresentation;
    if (!current || current.kind !== 'tool') return;

    const pinned = this.workspaceSurfaces.pin(current.workspaceId);
    if (!pinned) {
      await this.confirmDialog.alert({
        title: 'Pin Workspace Failed',
        message:
          'This workspace could not be persisted. Very large result sets may exceed local workspace storage capacity.',
      });
      return;
    }

    this.workspacePresentation = pinned;
    this.cdr.markForCheck();
  }

  openSearchContainingFolder(entry: FileInfo): void {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return;

    const target = entry.type === FileType.DIRECTORY
      ? entry.path
      : this.parentLocation(entry.path);

    pane.navigation?.navigateTo(target);
    this.cdr.markForCheck();
  }

  navigateFromTools(path: string): void {
    const pane = this.activePane?.contextState.isDirectoryTab
      ? this.activePane
      : this.otherPane?.contextState.isDirectoryTab
        ? this.otherPane
        : null;

    if (!pane || !path) return;
    pane.navigation?.navigateTo(path);
    this.cdr.markForCheck();
  }

  openTools(page: 'operations' | 'disk' | 'duplicates'): void {
    const rootPath = this.activePane?.contextState.isDirectoryTab
      ? this.activePane.contextState.path
      : this.otherPane?.contextState.isDirectoryTab
        ? this.otherPane.contextState.path
        : '';

    this.toolsCenter?.open(page, rootPath);
  }

  async batchRenameActiveSelection(): Promise<void> {
    const pane = this.activePane;

    if (!pane?.contextState.isDirectoryTab) {
      await this.confirmDialog.alert({
        title: 'Batch Rename',
        message: 'Batch Rename is available from a directory pane.',
      });
      return;
    }

    if (!pane.contextState.sourceCapabilities.rename) {
      await this.confirmDialog.alert({
        title: 'Batch Rename',
        message: 'This location does not allow renaming.',
      });
      return;
    }

    const selected = pane.selection.selectedEntries;
    if (selected.length < 2) {
      await this.confirmDialog.alert({
        title: 'Batch Rename',
        message:
          selected.length === 1
            ? 'Select at least one more item to batch rename.'
            : 'Select at least two items to batch rename.',
      });
      return;
    }

    const items: BatchRenameItem[] = selected.map(entry => ({
      entry,
      newName: entry.name,
    }));

    const renamed = await this.batchRenameDialog.openDialog(items, pane.contextState.path);
    if (!renamed?.length) return;

    const moves = await Promise.all(
      renamed.map(async item => ({
        source: item.entry.path,
        destination: await this.api.joinPaths([
          this.parentLocation(item.entry.path),
          item.newName,
        ]),
        exactDestination: true,
      })),
    );

    try {
      const result = await this.api.batchMove(moves, true);
      this.lastTransfer = `batch rename: ${result.succeeded}/${result.total}`;

      await this.refreshBothPanes();

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Batch Rename Completed with Errors',
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be renamed.`,
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Batch Rename Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async reopenLastClosedTab(): Promise<void> {
    const restored = this.workspaceService.reopenLastClosedTab(
      this.activePaneId,
    );
    if (!restored) return;

    this.activePaneId = restored.paneId;
    this.workspaceService.setActivePaneId(restored.paneId);

    const pane =
      restored.paneId === 'left'
        ? this.leftPane
        : this.rightPane;

    if (!pane) return;

    await pane.tabCommands?.reloadWorkspaceTabs(restored.tab.id);
    pane.navigation?.focusContent();
    this.closedTabsVisible = false;
    this.cdr.markForCheck();
  }

  async reopenClosedTabEntry(
    entry: RecentlyClosedTab,
  ): Promise<void> {
    const restored = this.workspaceService.reopenClosedTabById(entry.id);
    if (!restored) return;

    this.activePaneId = restored.paneId;
    this.workspaceService.setActivePaneId(restored.paneId);

    const pane =
      restored.paneId === 'left'
        ? this.leftPane
        : this.rightPane;

    if (!pane) return;

    await pane.tabCommands?.reloadWorkspaceTabs(restored.tab.id);
    pane.navigation?.focusContent();
    this.closedTabsVisible = false;
    this.cdr.markForCheck();
  }

  async moveActiveTabToOtherPane(): Promise<void> {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination) return;

    const tabId = source.tabCommands?.exportActiveTabId();
    if (!tabId) return;

    const toIndex = destination.tabCommands?.exportTabCount() ?? 0;
    const moved = this.workspaceService.moveTab(
      this.activePaneId,
      this.activePaneId === 'left' ? 'right' : 'left',
      tabId,
      toIndex,
    );
    if (!moved) return;

    const destinationId: PaneId =
      this.activePaneId === 'left' ? 'right' : 'left';

    await Promise.all([
      source.tabCommands?.reloadWorkspaceTabs(),
      destination.tabCommands?.reloadWorkspaceTabs(tabId),
    ]);

    this.activate(destinationId);
    this.cdr.markForCheck();
  }

  async reorderActiveTab(delta: -1 | 1): Promise<void> {
    const pane = this.activePane;
    if (!pane) return;

    const tabId = pane.tabCommands?.exportActiveTabId();
    if (!tabId) return;

    const currentCount = pane.tabCommands?.exportTabCount() ?? 0;
    const workspace = this.workspaceService.activeWorkspace$.value;
    const paneState =
      this.activePaneId === 'left'
        ? workspace?.leftPane
        : workspace?.rightPane;

    const currentIndex =
      paneState?.tabs?.findIndex((tab: any) => tab.id === tabId) ?? -1;
    if (currentIndex < 0) return;

    const targetIndex = Math.max(
      0,
      Math.min(Math.max(0, currentCount - 1), currentIndex + delta),
    );

    const moved = this.workspaceService.moveTab(
      this.activePaneId,
      this.activePaneId,
      tabId,
      targetIndex,
    );
    if (!moved) return;

    await pane.tabCommands?.reloadWorkspaceTabs(tabId);
    this.cdr.markForCheck();
  }

  async undoLastOperation(): Promise<void> {
    try {
      const label = await this.operationHistory.undo();
      if (!label) return;
      await this.refreshBothPanes();
      this.cdr.markForCheck();
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Undo Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async redoLastOperation(): Promise<void> {
    try {
      const label = await this.operationHistory.redo();
      if (!label) return;
      await this.refreshBothPanes();
      this.cdr.markForCheck();
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Redo Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async openTerminalHere(): Promise<void> {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab || pane.contextState.isInArchive || !pane.contextState.path) return;

    try {
      await this.api.openTerminalAt(pane.contextState.path);
      this.cdr.markForCheck();
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Open Terminal Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async refreshBothPanes(): Promise<void> {
    await Promise.all([
      this.leftPane?.navigation?.refresh(),
      this.rightPane?.navigation?.refresh(),
    ]);
    this.cdr.markForCheck();
  }

  private parentLocation(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const schemeIndex = normalized.indexOf('://');
    const minimum = schemeIndex >= 0 ? schemeIndex + 3 : 0;
    const slash = normalized.lastIndexOf('/');
    if (slash < minimum) return normalized;
    return normalized.slice(0, slash);
  }

  addActiveSelectionToBasket(): void {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return;

    const paths = pane.selection.selectedPaths.size > 0
      ? Array.from(pane.selection.selectedPaths)
      : pane.selection.cursorPath
        ? [pane.selection.cursorPath]
        : [];

    if (paths.length === 0) return;

    const added = this.selectionBasketService.add(
      paths,
      `pane:${this.activePaneId}`,
    );
    this.cdr.markForCheck();
  }

  async copyBasketTo(side: PaneId): Promise<void> {
    await this.transferBasket('copy', side);
  }

  async moveBasketTo(side: PaneId): Promise<void> {
    await this.transferBasket('move', side);
  }

  async deleteBasket(): Promise<void> {
    const paths = this.selectionBasketService.paths;
    if (paths.length === 0) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Delete Selection Basket',
      message:
        `Delete ${paths.length} basket item${paths.length === 1 ? '' : 's'}? ` +
        'Successful items will be removed from the basket; failed items will remain.',
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const result = await this.api.batchDelete(paths, true, true);

      // The batch result does not expose per-path successes in the lightweight
      // reassembly seam, so only clear automatically on complete success.
      if (result.failed === 0) {
        this.selectionBasketService.clear();
      }

      this.lastTransfer =
        `basket delete: ${result.succeeded}/${result.total}`;

      await Promise.all([
        this.leftPane?.navigation?.refresh(),
        this.rightPane?.navigation?.refresh(),
      ]);

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Basket Delete Completed with Errors',
          message:
            `${result.failed} item${result.failed === 1 ? '' : 's'} failed and remain in the basket.`,
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Basket Delete Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }

    this.cdr.markForCheck();
  }

  private async transferBasket(
    operation: 'copy' | 'move',
    destinationSide: PaneId,
  ): Promise<void> {
    const destination =
      destinationSide === 'left' ? this.leftPane : this.rightPane;

    if (!destination?.contextState.isDirectoryTab || !destination.contextState.path) return;

    const paths = this.selectionBasketService.paths;
    if (paths.length === 0) return;

    if (!destination.contextState.sourceCapabilities.create) {
      await this.confirmDialog.alert({
        title: 'Basket Transfer',
        message: 'The destination pane is read-only.',
      });
      return;
    }

    for (const sourcePath of paths) {
      const decision = canTransferBetween(operation, getSourceCapabilities(sourcePath), destination.contextState.sourceCapabilities);
      if (!decision.allowed) {
        await this.confirmDialog.alert(transferBlockedMessage(operation, decision));
        return;
      }
    }

    const confirmed = await this.confirmDialog.confirm({
      title: operation === 'copy' ? 'Copy Basket' : 'Move Basket',
      message:
        `${operation === 'copy' ? 'Copy' : 'Move'} ${paths.length} basket item${
          paths.length === 1 ? '' : 's'
        } to the ${destinationSide} pane?`,
      confirmText: operation === 'copy' ? 'Copy' : 'Move',
    });
    if (!confirmed) return;

    try {
      const items = await Promise.all(
        paths.map(async source => ({
          source,
          destination: await this.api.joinPaths([
            destination.contextState.path,
            this.leafName(source),
          ]),
        })),
      );

      const result = operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      this.lastTransfer =
        `basket ${operation}: ${result.succeeded}/${result.total}`;

      await destination.navigation?.refresh();

      // On a complete move, the basket no longer represents existing source
      // locations. Copies intentionally remain in the working set.
      if (operation === 'move' && result.failed === 0) {
        this.selectionBasketService.clear();
        await Promise.all([
          this.leftPane?.navigation?.refresh(),
          this.rightPane?.navigation?.refresh(),
        ]);
      }

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: `${operation === 'copy' ? 'Copy' : 'Move'} Completed with Errors`,
          message:
            `${result.failed} item${result.failed === 1 ? '' : 's'} could not be transferred. ` +
            'The basket has been kept intact.',
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Basket Transfer Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }

    this.cdr.markForCheck();
  }

  canComparePanes(): boolean {
    return !!this.leftPane &&
      !!this.rightPane &&
      this.leftPane.contextState.isDirectoryTab &&
      this.rightPane.contextState.isDirectoryTab &&
      !!this.leftPane.contextState.path &&
      !!this.rightPane.contextState.path;
  }

  get leftComparisonStatuses() {
    return this.paneComparison.statusMaps(this.paneComparisonResult).left;
  }

  get rightComparisonStatuses() {
    return this.paneComparison.statusMaps(this.paneComparisonResult).right;
  }
  get paneComparisonResult(): PaneComparisonResult {
    const recursiveCurrent =
      this.comparisonRecursive &&
      !!this.recursiveComparisonResult &&
      !!this.recursiveComparisonRoots &&
      this.recursiveComparisonRoots.left === this.leftPane?.contextState.path &&
      this.recursiveComparisonRoots.right === this.rightPane?.contextState.path;

    if (this.verifiedComparisonResult && !this.comparisonRecursive) {
      return this.verifiedComparisonResult;
    }

    if (recursiveCurrent) {
      return this.recursiveComparisonResult!;
    }

    return this.paneComparison.compare(
      this.leftPane?.contextState.entries ?? [],
      this.rightPane?.contextState.entries ?? [],
      { policy: this.comparisonPolicy },
    );
  }

  async setComparisonPolicy(policy: PaneComparisonPolicy): Promise<void> {
    this.comparisonPolicy = policy;
    this.verifiedComparisonResult = null;
    this.syncReviewVisible = false;

    if (this.comparisonRecursive) {
      await this.refreshRecursiveComparison();
      if (this.recursiveComparisonResult && (policy === 'hash' || policy === 'content')) {
        this.recursiveComparisonScanning = true;
        this.cdr.markForCheck();
        try {
          this.recursiveComparisonResult = await this.paneComparison.verify(this.api, this.recursiveComparisonResult, policy);
        } finally {
          this.recursiveComparisonScanning = false;
          this.cdr.markForCheck();
        }
      }
      return;
    }

    const base = this.paneComparison.compare(
      this.leftPane?.contextState.entries ?? [],
      this.rightPane?.contextState.entries ?? [],
      { policy },
    );
    if (policy === 'hash' || policy === 'content') {
      this.recursiveComparisonScanning = true;
      this.cdr.markForCheck();
      try {
        this.verifiedComparisonResult = await this.paneComparison.verify(this.api, base, policy);
      } finally {
        this.recursiveComparisonScanning = false;
      }
    } else {
      this.verifiedComparisonResult = base;
    }
    this.cdr.markForCheck();
  }

  togglePaneComparison(): void {
    if (this.comparisonVisible) {
      this.closePaneComparison();
      return;
    }

    if (!this.canComparePanes()) {
      void this.confirmDialog.alert({
        title: 'Compare Panes',
        message: 'Pane comparison is available when both panes show directories.',
      });
      return;
    }

    this.comparisonVisible = true;
    this.cdr.markForCheck();
  }

  closePaneComparison(): void {
    this.comparisonVisible = false;
    this.syncReviewVisible = false;
    this.comparisonRecursive = false;
    this.recursiveComparisonResult = null;
    this.verifiedComparisonResult = null;
    this.recursiveComparisonError = '';
    this.cdr.markForCheck();
  }

  openSyncReview(): void {
    if (!this.canComparePanes()) return;
    this.comparisonVisible = true;
    this.syncReviewVisible = true;
    this.cdr.markForCheck();
  }

  closeSyncReview(): void {
    this.syncReviewVisible = false;
    this.cdr.markForCheck();
  }

  async toggleRecursiveComparison(): Promise<void> {
    if (!this.canComparePanes()) return;

    this.comparisonRecursive = !this.comparisonRecursive;
    this.syncReviewVisible = false;

    if (this.comparisonRecursive) {
      await this.refreshRecursiveComparison();
    } else {
      this.recursiveComparisonResult = null;
      this.recursiveComparisonRoots = null;
      this.recursiveComparisonError = '';
      this.cdr.markForCheck();
    }
  }

  async refreshRecursiveComparison(): Promise<void> {
    if (!this.canComparePanes() || !this.leftPane || !this.rightPane) return;

    this.recursiveComparisonScanning = true;
    this.recursiveComparisonError = '';
    this.cdr.markForCheck();

    try {
      const result = await this.recursivePaneComparison.compare(
        this.api,
        this.leftPane.contextState.path,
        this.rightPane.contextState.path,
        { showHidden: this.showHidden },
      );

      this.recursiveComparisonResult = (this.comparisonPolicy === 'hash' || this.comparisonPolicy === 'content')
        ? await this.paneComparison.verify(this.api, result.comparison, this.comparisonPolicy)
        : this.paneComparison.fromItems(result.comparison.items, this.comparisonPolicy);
      this.recursiveComparisonRoots = {
        left: this.leftPane.contextState.path,
        right: this.rightPane.contextState.path,
      };

      if (result.truncated) {
        this.recursiveComparisonError =
          'Scan limit reached; recursive results are partial.';
      } else if (result.warnings.length > 0) {
        this.recursiveComparisonError =
          `${result.warnings.length} folder${result.warnings.length === 1 ? '' : 's'} could not be scanned.`;
      }
    } catch (error) {
      this.recursiveComparisonResult = null;
      this.recursiveComparisonRoots = null;
      this.recursiveComparisonError =
        error instanceof Error ? error.message : String(error);
    } finally {
      this.recursiveComparisonScanning = false;
      this.cdr.markForCheck();
    }
  }

  async refreshComparison(): Promise<void> {
    if (!this.canComparePanes()) return;

    await Promise.all([
      this.leftPane?.navigation?.refresh(),
      this.rightPane?.navigation?.refresh(),
    ]);

    if (this.comparisonRecursive) {
      await this.refreshRecursiveComparison();
    }

    this.cdr.markForCheck();
  }

  addComparisonDifferencesToBasket(side: PaneId | 'both'): void {
    if (!this.canComparePanes()) return;

    const result = this.paneComparisonResult;
    const paths = new Set<string>();

    for (const item of result.items) {
      if (
        (side === 'left' || side === 'both') &&
        item.left &&
        item.status !== 'same'
      ) {
        paths.add(item.left.path);
      }

      if (
        (side === 'right' || side === 'both') &&
        item.right &&
        item.status !== 'same'
      ) {
        paths.add(item.right.path);
      }
    }

    if (paths.size === 0) return;

    this.selectionBasketService.add(
      Array.from(paths),
      `comparison:${side}`,
    );
    this.selectionBasket?.show();
    this.cdr.markForCheck();
  }

  selectComparisonDifferences(side: PaneId): void {
    if (!this.canComparePanes()) return;

    const result = this.paneComparisonResult;
    const paths = side === 'left'
      ? this.paneComparison.leftDifferencePaths(result)
      : this.paneComparison.rightDifferencePaths(result);

    const pane = side === 'left' ? this.leftPane : this.rightPane;
    pane?.selection?.selectPaths(paths);
    this.activate(side);
  }

  async copyMissingComparison(sourceSide: PaneId): Promise<void> {
    if (!this.canComparePanes()) return;

    const result = this.paneComparisonResult;
    const missing = result.items.filter(item =>
      sourceSide === 'left'
        ? item.status === 'only-left' && !!item.left
        : item.status === 'only-right' && !!item.right,
    );

    if (missing.length === 0) return;

    const sourcePane = sourceSide === 'left' ? this.leftPane : this.rightPane;
    const destinationPane =
      sourceSide === 'left' ? this.rightPane : this.leftPane;
    if (!sourcePane || !destinationPane) return;

    const items = await Promise.all(
      missing.map(async item => {
        const source =
          sourceSide === 'left' ? item.left!.path : item.right!.path;
        const relativePath = this.comparisonRecursive
          ? item.key
          : this.leafName(source);

        return {
          source,
          destination: await this.api.joinPaths([
            destinationPane.contextState.path,
            relativePath,
          ]),
        };
      }),
    );

    await this.executeComparisonCopies(
      items,
      `Copy ${missing.length} missing item${missing.length === 1 ? '' : 's'} ${
        sourceSide === 'left' ? 'left в†’ right' : 'right в†’ left'
      }?`,
    );
  }

  async executeSyncPlan(plan: PaneSyncPlan): Promise<void> {
    if (!this.canComparePanes() || !plan) return;

    const approved = plan.items.filter(item => item.direction !== 'skip');
    if (approved.length === 0) return;

    const directionLabel = plan.preset === 'bidirectional'
      ? 'bidirectional'
      : plan.preset === 'left-to-right'
        ? 'left в†’ right'
        : plan.preset === 'right-to-left'
          ? 'right в†’ left'
          : 'custom';
    const conflictNote = plan.conflicts > 0
      ? ` ${plan.conflicts} comparison conflict${plan.conflicts === 1 ? '' : 's'} remain explicitly reviewable and skipped unless you selected a direction.`
      : '';
    const confirmed = await this.confirmDialog.confirm({
      title: 'Execute Synchronization Plan',
      message:
        `Apply ${approved.length} approved synchronization action${approved.length === 1 ? '' : 's'} (${directionLabel})?` +
        `${conflictNote} Destination conflicts use the normal conflict-resolution workflow.`,
      confirmText: 'Synchronize',
    });
    if (!confirmed) return;

    const startedAt = Date.now();
    const items: Array<{ source: string; destination: string }> = [];

    for (const item of approved) {
      const leftToRight = item.direction === 'left-to-right';
      const source = leftToRight ? item.leftPath : item.rightPath;
      const destinationRoot = leftToRight
        ? this.rightPane?.contextState.path
        : this.leftPane?.contextState.path;

      if (!source || !destinationRoot) continue;

      const relativePath = this.comparisonRecursive
        ? item.key
        : this.leafName(source);

      items.push({
        source,
        destination: await this.api.joinPaths([
          destinationRoot,
          relativePath,
        ]),
      });
    }

    if (items.length === 0) return;

    const result = await this.api.batchCopy(items, true);
    const mode = plan.preset === 'left-to-right' || plan.preset === 'right-to-left' || plan.preset === 'bidirectional'
      ? plan.preset
      : 'custom';
    this.operationHistory.recordSync({
      startedAt,
      completedAt: Date.now(),
      leftRoot: this.leftPane?.contextState.path ?? '',
      rightRoot: this.rightPane?.contextState.path ?? '',
      mode,
      recursive: this.comparisonRecursive,
      planned: items.length,
      succeeded: result.succeeded,
      skipped: result.skipped,
      failed: result.failed,
      conflicts: plan.conflicts,
    });
    this.lastTransfer = `sync ${mode}: ${result.succeeded}/${result.total}`;

    this.syncReviewVisible = false;
    await Promise.all([
      this.leftPane?.navigation?.refresh(),
      this.rightPane?.navigation?.refresh(),
    ]);

    if (this.comparisonRecursive) {
      await this.refreshRecursiveComparison();
    }

    if (result.failed > 0) {
      await this.confirmDialog.alert({
        title: 'Synchronization Completed with Errors',
        message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be synchronized.`,
      });
    }

    this.cdr.markForCheck();
  }

  private async executeComparisonCopies(
    items: Array<{ source: string; destination: string }>,
    confirmationMessage: string,
  ): Promise<void> {
    const confirmed = await this.confirmDialog.confirm({
      title: 'Pane Synchronization',
      message: confirmationMessage,
      confirmText: 'Copy',
    });
    if (!confirmed) return;

    const result = await this.api.batchCopy(items, true);
    this.lastTransfer = `comparison copy: ${result.succeeded}/${result.total}`;

    await Promise.all([
      this.leftPane?.navigation?.refresh(),
      this.rightPane?.navigation?.refresh(),
    ]);

    if (this.comparisonRecursive) {
      await this.refreshRecursiveComparison();
    }

    if (result.failed > 0) {
      await this.confirmDialog.alert({
        title: 'Copy Completed with Errors',
        message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be copied.`,
      });
    }

    this.cdr.markForCheck();
  }

  async extractArchiveSelectionToOther(): Promise<void> {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source?.contextState.isInArchive || !destination?.contextState.isDirectoryTab) return;

    const extracted = await source.commands?.extractSelectedTo(destination.contextState.path);
    if (!extracted) return;

    this.lastTransfer = 'archive extract to other pane';
    await destination.navigation?.refresh();
    this.cdr.markForCheck();
  }

  async extractArchiveSelectionToFolder(): Promise<void> {
    const source = this.activePane;
    if (!source?.contextState.isInArchive) return;

    const suggested = this.otherPane?.contextState.isDirectoryTab
      ? this.otherPane.contextState.path
      : '';

    const destination = (await this.confirmDialog.prompt({
      title: 'Extract Archive Entries',
      message: 'Extract selected archive entries to:',
      confirmText: 'Extract',
      defaultValue: suggested,
      historyKey: 'reassembly.archive.extract.destination',
    }))?.trim();

    if (!destination) return;

    const extracted = await source.commands?.extractSelectedTo(destination);
    if (!extracted) return;

    if (
      this.leftPane?.contextState.path === destination ||
      this.rightPane?.contextState.path === destination
    ) {
      await this.refreshBothPanes();
    }

    this.lastTransfer = `archive extract -> ${destination}`;
    this.cdr.markForCheck();
  }

  async copyToOther(): Promise<void> {
    await this.transferToOther('copy');
  }

  async moveToOther(): Promise<void> {
    await this.transferToOther('move');
  }

  async handleInternalDrop(
    targetPaneId: PaneId,
    event: DragEvent,
    targetPath: string,
  ): Promise<void> {
    const payload = this.readInternalDragPayload(event);
    if (!payload || payload.entries.length === 0 || !targetPath) return;

    const targetPane = targetPaneId === 'left' ? this.leftPane : this.rightPane;
    if (!targetPane) return;

    const sourcePane = payload.sourcePaneId === 'left' ? this.leftPane : this.rightPane;
    const operation = dragOperationFromModifiers(event);

    const sourceCapabilities = sourcePane?.contextState.sourceCapabilities ?? {
      read: true,
      create: false,
      delete: operation === 'move',
      rename: false,
      trash: false,
    };

    const decision = canTransferBetween(
      operation,
      sourceCapabilities,
      targetPane.contextState.sourceCapabilities,
    );

    if (!decision.allowed) {
      await this.confirmDialog.alert(
        transferBlockedMessage(operation, decision),
      );
      return;
    }

    const entries = collapseNestedDragEntries(payload.entries);
    const sourcePaths = entries.map(entry => entry.path).filter(path => !!path);
    if (sourcePaths.length === 0) return;

    const dropDecision = decideInternalDrop({
      entries,
      sourceDirectory: sourcePane?.contextState.path ?? payload.sourceDirectory ?? '',
      targetDirectory: targetPath,
      operation,
    });

    if (dropDecision.validity !== 'allowed') {
      this.lastDrop = `ignored: ${dropDecision.reason.toLocaleLowerCase()}`;
      this.cdr.markForCheck();
      return;
    }

    try {
      if (sourcePane?.contextState.isInArchive) {
        const result = await sourcePane.commands?.transferArchiveSelectionTo(
          targetPath,
          operation,
        );
        if (!result) return;

        this.lastDrop =
          `archive ${operation} drop: ${result.succeeded}/${result.total}`;
        this.lastTransfer = this.lastDrop;

        await targetPane.navigation?.refresh();

        if (operation === 'move' && result.sourceRemoved) {
          await sourcePane.navigation?.refresh();
        }

        if (result.failed > 0) {
          await this.confirmDialog.alert({
            title: 'Archive Drop Completed with Errors',
            message:
              `${result.failed} archive entr${
                result.failed === 1 ? 'y' : 'ies'
              } could not be transferred.`,
          });
        }
        return;
      }

      const items = await Promise.all(
        sourcePaths.map(async sourcePath => ({
          source: sourcePath,
          destination: await this.api.joinPaths([
            targetPath,
            this.leafName(sourcePath),
          ]),
        })),
      );

      const result = operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      const affected = result.affectedPaths ?? [];
      if (
        result.failed === 0 &&
        result.skipped === 0 &&
        affected.length === result.succeeded &&
        affected.length > 0
      ) {
        this.operationHistory.record({
          label: `${operation === 'copy' ? 'Copy' : 'Move'} ${affected.length} dropped item${
            affected.length === 1 ? '' : 's'
          }`,
          undo: async () => {
            if (operation === 'copy') {
              const deletion = await this.api.batchDelete(
                affected.map(item => item.destination),
                true,
                false,
              );
              if (deletion.failed > 0) throw new Error('Undo drop copy failed.');
              return;
            }
            const back = await this.api.batchMove(
              affected.map(item => ({
                source: item.destination,
                destination: item.source,
                exactDestination: true,
              })),
              true,
            );
            if (back.failed > 0 || back.skipped > 0) {
              throw new Error('Undo drop move failed.');
            }
          },
          redo: async () => {
            const forward = affected.map(item => ({
              source: item.source,
              destination: item.destination,
              exactDestination: true,
            }));
            const redone = operation === 'copy'
              ? await this.api.batchCopy(forward, true)
              : await this.api.batchMove(forward, true);
            if (redone.failed > 0 || redone.skipped > 0) {
              throw new Error(`Redo drop ${operation} failed.`);
            }
          },
        });
      }

      this.lastDrop =
        `${operation} drop: ${result.succeeded}/${result.total}` +
        (result.skipped > 0 ? ` · ${result.skipped} skipped` : '') +
        (result.failed > 0 ? ` · ${result.failed} failed` : '');
      this.lastTransfer = this.lastDrop;

      await targetPane.navigation?.refresh();

      if (
        operation === 'move' &&
        sourcePane &&
        sourcePane !== targetPane
      ) {
        await sourcePane.navigation?.refresh();
      } else if (
        operation === 'move' &&
        sourcePane &&
        sourcePane === targetPane &&
        targetPath !== sourcePane.contextState.path
      ) {
        await sourcePane.navigation?.refresh();
      }

      if (result.failed > 0 || result.skipped > 0) {
        await this.confirmDialog.alert({
          title: 'Drop Completed with Issues',
          message: [
            result.failed > 0
              ? `${result.failed} item${result.failed === 1 ? '' : 's'} failed.`
              : '',
            result.skipped > 0
              ? `${result.skipped} item${result.skipped === 1 ? '' : 's'} were skipped.`
              : '',
          ].filter(Boolean).join('\n'),
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Drag & Drop Error',
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      this.cdr.markForCheck();
    }
  }

  private readInternalDragPayload(event: DragEvent): {
    sourcePaneId: PaneId;
    sourceDirectory: string;
    entries: Array<{ path: string; name?: string; type?: unknown }>;
  } | null {
    try {
      const raw = event.dataTransfer?.getData('application/json');
      if (!raw) return null;

      const parsed = JSON.parse(raw) as {
        sourcePaneId?: unknown;
        sourceDirectory?: unknown;
        entries?: unknown;
      };

      const sourcePaneId: PaneId =
        parsed.sourcePaneId === 'right' ? 'right' : 'left';

      const entries = Array.isArray(parsed.entries)
        ? parsed.entries.filter(
            (entry): entry is { path: string; name?: string; type?: unknown } =>
              !!entry &&
              typeof entry === 'object' &&
              typeof (entry as { path?: unknown }).path === 'string',
          )
        : [];

      return {
        sourcePaneId,
        sourceDirectory:
          typeof parsed.sourceDirectory === 'string'
            ? parsed.sourceDirectory
            : '',
        entries,
      };
    } catch {
      return null;
    }
  }

  openEntryMenu(
    paneId: PaneId,
    event: MouseEvent,
    entry: FileInfo,
  ): void {
    this.captureContextMenuFocus();
    this.activate(paneId, false);

    const pane = this.activePane;

    if (/^registry:\/\//i.test(pane?.contextState.path ?? '')) {
      const pluginItems = this.pluginContextMenuItems(pane, entry.path);
      this.contextMenu = {
        x: event.clientX,
        y: event.clientY,
        title: entry.name || 'Registry entry',
        items: normalizeContextMenuItems(pluginItems),
      };
      this.cdr.markForCheck();
      return;
    }
    const isParent = entry.path === '__parent__';
    const isDirectory = entry.type === FileType.DIRECTORY;
    const isArchive = entry.type === FileType.ARCHIVE;
    const selectionCount = pane?.selection.selectedEntries.length ?? 0;
    const canRename = !!pane?.mutationState.canRename;
    const canMutate = !!pane?.contextState.isDirectoryTab && !pane.contextState.isInArchive;
    const canCreate = !!pane?.mutationState.canCreate;
    const canDelete = !!pane?.mutationState.canDelete;

    const items: ContextMenuItem[] = [];

    this.pushMenuSection(items, [
      this.menuCommand(COMMAND.open, isParent ? 'Go to parent folder' : 'Open', 'Enter'),
      ...(!isParent && !isDirectory ? [
        this.menuCommand(COMMAND.quickView, 'Quick view', 'F3'),
        this.menuCommand(COMMAND.openWithExternal, 'Open with default app', 'Ctrl+Shift+O'),
        this.menuCommand(COMMAND.openWithChooser, 'Open With…'),
      ] : []),
    ]);

    if (!isParent) {
      this.pushMenuSection(items, [
        ...(canRename ? [this.menuCommand(COMMAND.rename, 'Rename', 'F2')] : []),
        ...(selectionCount > 1 && !!pane?.contextState.sourceCapabilities.rename
          ? [this.menuCommand(COMMAND.batchRename, 'Batch rename…', 'Ctrl+Shift+R')]
          : []),
        this.menuCommand(COMMAND.copy, 'Copy', 'Ctrl+C'),
        ...(canMutate ? [this.menuCommand(COMMAND.cut, 'Cut', 'Ctrl+X')] : []),
        ...(canCreate ? [this.menuCommand(COMMAND.duplicate, 'Duplicate', 'Ctrl+D')] : []),
        ...(canCreate && selectionCount > 0
          ? [this.menuCommand(COMMAND.moveSelectionToNewFolder, 'Move Selection to New Folder')]
          : []),
        this.menuCommand(COMMAND.copyToOther, 'Copy to other pane', 'F5'),
        ...(canMutate ? [this.menuCommand(COMMAND.moveToOther, 'Move to other pane', 'F6')] : []),
      ]);

      this.pushMenuSection(items, [
        this.menuCommand(COMMAND.copyPath, 'Copy Path', 'Ctrl+Shift+C'),
        this.menuCommand(COMMAND.copyRelativePath, 'Copy Relative Path', 'Ctrl+Alt+C'),
        this.menuCommand(COMMAND.copyFileUri, 'Copy File URI'),
        this.menuCommand(COMMAND.copyName, 'Copy Name'),
        this.menuCommand(COMMAND.revealInFileManager, 'Reveal in System File Manager'),
      ]);

      this.pushMenuSection(items, [
        this.menuCommand(COMMAND.properties, 'Properties', 'Alt+Enter'),
        this.menuCommand(COMMAND.detailsWorkspace, 'Open Full Details Workspace'),
        this.menuCommand(COMMAND.calculateSize, 'Calculate Size'),
        this.menuCommand(COMMAND.attributes, 'Attributes / Permissions'),
        this.menuCommand(COMMAND.setModifiedTime, 'Set Modified Time…'),
        ...(!isDirectory ? [
          this.menuCommand(COMMAND.checksum, 'SHA-256 Checksum'),
          this.menuCommand(COMMAND.splitFile, 'Split File…'),
        ] : []),
        ...(selectionCount >= 2
          ? [this.menuCommand(COMMAND.joinFiles, 'Join Selected File Parts…')]
          : []),
      ]);

      this.pushMenuSection(items, [
        ...(!isDirectory ? [
          this.menuCommand(COMMAND.selectSameExtension, 'Select Same Extension', 'Alt+E'),
          this.menuCommand(COMMAND.selectSameType, 'Select Same Type'),
        ] : [this.menuCommand(COMMAND.selectSameType, 'Select Same Type')]),
        this.menuCommand(COMMAND.addToBasket, 'Add to Selection Basket', 'Ctrl+Alt+B'),
      ]);

      if (canCreate) {
        this.pushMenuSection(items, [
          this.menuCommand(COMMAND.createShortcut, 'Create Shortcut'),
          this.menuCommand(COMMAND.createSymlink, 'Create Symbolic Link'),
          ...(!isDirectory ? [this.menuCommand(COMMAND.createHardlink, 'Create Hard Link')] : []),
          this.menuCommand(COMMAND.createArchive, 'Create ZIP Archive', 'Alt+F5'),
        ]);
      }

      if (isArchive) {
        this.pushMenuSection(items, [
          this.menuCommand(COMMAND.extractArchiveToOther, 'Extract to other pane'),
          this.menuCommand(COMMAND.extractArchiveSelection, 'Extract to folder…'),
        ]);
      }
    }

    const pluginItems = this.pluginContextMenuItems(pane, entry.path);
    if (pluginItems.length) this.pushMenuSection(items, pluginItems);

    if (!isParent && canDelete) {
      this.pushMenuSection(items, [
        this.menuCommand(COMMAND.deleteSelected, 'Delete', 'Delete', { danger: true }),
        this.menuCommand(COMMAND.deletePermanent, 'Delete permanently', 'Shift+Delete', { danger: true }),
      ]);
    }

    this.contextMenu = {
      x: event.clientX,
      y: event.clientY,
      title: isParent ? 'Parent folder' : this.entryContextTitle(entry, selectionCount),
      items: normalizeContextMenuItems(items),
    };
    this.cdr.markForCheck();
  }

  openNavigationTrail(
    paneId: PaneId,
    request: {
      direction: 'back' | 'forward';
      event: MouseEvent;
      entries: readonly string[];
    },
  ): void {
    this.captureContextMenuFocus();
    this.activate(paneId, false);
    this.navigationTrailCommands.clear();

    const items: ContextMenuItem[] = request.entries.map((path, index) => {
      const commandId = `reassembly.navigation.trail.${paneId}.${index}`;
      this.navigationTrailCommands.set(commandId, { paneId, path });
      return {
        commandId,
        label: this.navigationTrailLabel(path),
        shortcut: index === 0
          ? (request.direction === 'back' ? 'Alt+Left' : 'Alt+Right')
          : undefined,
        enabled: true,
      };
    });

    this.contextMenu = {
      x: request.event.clientX,
      y: request.event.clientY,
      title: request.direction === 'back' ? 'Back history' : 'Forward history',
      items,
    };
    this.cdr.markForCheck();
  }

  private navigationTrailLabel(path: string): string {
    const normalized = (path || '').replace(/\\/g, '/').replace(/\/+$/, '');
    const parts = normalized.split('/').filter(Boolean);
    const leaf = parts[parts.length - 1] || normalized || path;
    return leaf === path ? leaf : `${leaf}  вЂ”  ${path}`;
  }

  openBackgroundMenu(paneId: PaneId, event: MouseEvent): void {
    this.captureContextMenuFocus();
    this.activate(paneId, false);

    const pane = this.activePane;

    if (/^registry:\/\//i.test(pane?.contextState.path ?? '')) {
      const pluginItems = this.pluginContextMenuItems(
        pane,
        pane?.contextState.path ?? '',
      );
      this.contextMenu = {
        x: event.clientX,
        y: event.clientY,
        title: pane?.contextState.path || 'Registry',
        items: normalizeContextMenuItems(pluginItems),
      };
      this.cdr.markForCheck();
      return;
    }
    const items: ContextMenuItem[] = [];

    this.pushMenuSection(items, [
      ...(pane?.mutationState.canCreate ? [
        this.menuCommand(COMMAND.newFolder, 'New Folder', 'F7'),
        this.menuCommand(COMMAND.newFile, 'New File'),
        this.menuCommand(COMMAND.newTextDocument, 'New Text Document'),
        this.menuCommand(COMMAND.newMarkdownDocument, 'New Markdown Document'),
        this.menuCommand(COMMAND.newJsonDocument, 'New JSON Document'),
      ] : []),
      this.menuCommand(COMMAND.paste, 'Paste', 'Ctrl+V'),
      this.menuCommand(COMMAND.undo, 'Undo', 'Ctrl+Z'),
      this.menuCommand(COMMAND.redo, 'Redo', 'Ctrl+Y'),
    ]);

    this.pushMenuSection(items, [
      this.menuCommand(COMMAND.selectAll, 'Select All', 'Ctrl+A'),
      this.menuCommand(COMMAND.selectFiles, 'Select Visible Files', 'Alt+Shift+F'),
      this.menuCommand(COMMAND.selectFolders, 'Select Visible Folders', 'Alt+Shift+D'),
      this.menuCommand(COMMAND.clearSelection, 'Clear Selection', 'Ctrl+Alt+D'),
      this.menuCommand(COMMAND.invertSelection, 'Invert Selection', 'Alt+I'),
      this.menuCommand(COMMAND.selectByMask, 'Select by Mask…', 'Alt+S'),
      this.menuCommand(COMMAND.unselectByMask, 'Unselect by Mask…', 'Alt+U'),
      this.menuCommand(COMMAND.openBasket, 'Selection Basket', 'Ctrl+Shift+B'),
    ]);

    this.pushMenuSection(items, [
      this.menuCommand(
        COMMAND.toggleCurrentPin,
        this.isPanePathPinned(pane) ? 'Unpin this Location' : 'Pin this Location',
      ),
      this.menuCommand(COMMAND.globalSearch, 'Search Here…', 'Ctrl+K'),
      this.menuCommand(COMMAND.quickPathJump, 'Go to Path…', 'Ctrl+P'),
      this.menuCommand(COMMAND.refresh, 'Refresh', 'Ctrl+R'),
      this.menuCommand(COMMAND.openTerminal, 'Open Terminal Here', 'Ctrl+Alt+T'),
      this.menuCommand(COMMAND.diskUsage, 'Disk Usage', 'Ctrl+Shift+D'),
      this.menuCommand(COMMAND.findDuplicates, 'Find Duplicates'),
      this.menuCommand(COMMAND.remoteConnect, 'Connect Remote…'),
    ]);

    this.pushMenuSection(items, [
      this.menuCommand(COMMAND.toggleTabPin, 'Pin / Unpin Current Tab', 'Ctrl+Alt+P'),
      this.menuCommand(COMMAND.moveTabToOtherPane, 'Move Current Tab to Other Pane'),
      this.menuCommand(COMMAND.moveTabLeft, 'Move Tab Left'),
      this.menuCommand(COMMAND.moveTabRight, 'Move Tab Right'),
      this.menuCommand(COMMAND.closeOtherTabs, 'Close Other Tabs'),
      this.menuCommand(COMMAND.closeTabsToRight, 'Close Tabs to the Right'),
    ]);

    this.pushMenuSection(items, [
      this.menuCommand(COMMAND.openOperations, 'Operations Center'),
      this.menuCommand(COMMAND.openTrash, 'Trash / Recycle Bin'),
      this.menuCommand(COMMAND.openCommandPalette, 'Command Palette', 'Ctrl+Shift+P'),
      this.menuCommand(COMMAND.openSettings, 'Settings', 'Ctrl+,'),
      this.menuCommand(COMMAND.openPluginSettings, 'Plugin Settings'),
    ]);

    const pluginItems = this.pluginContextMenuItems(
      pane,
      pane?.contextState.path ?? '',
    );
    if (pluginItems.length) this.pushMenuSection(items, pluginItems);

    this.contextMenu = {
      x: event.clientX,
      y: event.clientY,
      title: pane?.contextState.path || 'Folder',
      items: normalizeContextMenuItems(items),
    };
    this.cdr.markForCheck();
  }

  dispatchContextCommand(commandId: string): void {
    const trail = this.navigationTrailCommands.get(commandId);
    if (trail) {
      this.closeContextMenu(false);
      this.navigationTrailCommands.clear();
      const pane = trail.paneId === 'left' ? this.leftPane : this.rightPane;
      void pane?.navigation?.jumpNavigationHistory(trail.path);
      return;
    }

    const contextualAction = this.contextMenuActions.get(commandId);
    if (contextualAction) {
      this.contextMenuActions.delete(commandId);
      this.navigationTrailCommands.clear();
      this.closeContextMenu(false);
      void Promise.resolve(contextualAction()).catch(error => {
        console.error('Context-menu action failed.', error);
      });
      return;
    }

    this.contextMenuActions.clear();
    this.navigationTrailCommands.clear();

    // Do not restore the old focus here: commands such as Rename deliberately
    // establish a new focus target of their own.
    this.closeContextMenu(false);
    this.input.dispatchCommandById(commandId, {
      source: 'context-menu',
      context: {
        paneId: this.activePaneId,
        path: this.activePane?.contextState.path ?? '',
        selectedPaths: this.pluginSelectionPaths(),
      },
    });
  }

  closeContextMenu(restoreFocus = true): void {
    if (!this.contextMenu) return;
    this.contextMenu = null;
    this.contextMenuActions.clear();
    const target = this.contextMenuReturnFocus;
    this.contextMenuReturnFocus = null;
    this.cdr.markForCheck();

    if (!restoreFocus) return;
    requestAnimationFrame(() => {
      if (target?.isConnected) {
        target.focus({ preventScroll: true });
      } else {
        this.activePane?.navigation?.focusContent();
      }
    });
  }

  private menuCommand(
    commandId: string,
    label: string,
    shortcut?: string,
    options: { danger?: boolean } = {},
  ): ContextMenuItem {
    const state = this.input.getCommandState(commandId);
    return {
      commandId,
      label,
      shortcut,
      enabled: !!state?.registered && state.enabled,
    };
  }

  private pushMenuSection(
    target: ContextMenuItem[],
    section: readonly ContextMenuItem[],
  ): void {
    const meaningful = section.filter(item => !!item && !item.separator);
    if (!meaningful.length) return;
    if (target.length) target.push({ separator: true });
    target.push(...meaningful);
  }

  private entryContextTitle(entry: FileInfo, selectionCount: number): string {
    if (selectionCount > 1) return `${selectionCount} selected · ${entry.name}`;
    return entry.name;
  }

  private captureContextMenuFocus(): void {
    this.contextMenuActions.clear();
    if (typeof document === 'undefined') {
      this.contextMenuReturnFocus = null;
      return;
    }
    const active = document.activeElement;
    this.contextMenuReturnFocus = active instanceof HTMLElement ? active : null;
  }


  isPanePathPinned(
    pane: PaneHostComponent | undefined,
  ): boolean {
    const location = pane?.pinState.location?.trim();
    if (!location) return false;

    return this.pinnedLocations.isPinned(
      this.pinnedLocations.load(),
      location,
      pane?.pinState.surfaceId || 'dir',
    );
  }

  togglePanePin(
    pane: PaneHostComponent | undefined,
  ): void {
    const location = pane?.pinState.location?.trim();
    if (!pane || !location) return;

    const surfaceId = pane.pinState.surfaceId || 'dir';
    let pins = this.pinnedLocations.load();
    const pinned = this.pinnedLocations.isPinned(
      pins,
      location,
      surfaceId,
    );

    if (pinned) {
      pins = this.pinnedLocations.remove(
        pins,
        location,
        surfaceId,
      );
    } else {
      const pin = this.pinnedLocations.create(
        location,
        pane.pinState.title,
        surfaceId === 'dir' ? undefined : 'в—†',
        Date.now(),
        surfaceId,
      );
      if (!pin) return;

      pins = [
        ...pins.filter(existing => existing.id !== pin.id),
        pin,
      ];
    }

    this.pinnedLocations.save(pins);
    this.cdr.markForCheck();
  }

  navigateFromHistory(path: string): void {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return;

    pane.navigation?.navigateTo(path);
    this.cdr.markForCheck();
  }

  navigateFromQuickAccess(path: string): void {
    const pane = this.activePane;
    if (!pane?.contextState.isDirectoryTab) return;

    pane.navigation?.navigateTo(path);
    this.cdr.markForCheck();
  }

  async openQuickAccessTarget(
    target: QuickAccessTarget,
  ): Promise<void> {
    const pane = this.activePane;
    if (!pane) return;

    const surfaceId = target.surfaceId?.trim() || 'dir';
    if (surfaceId === 'dir') {
      if (!pane.contextState.isDirectoryTab) return;
      pane.navigation?.navigateTo(target.path);
      this.cdr.markForCheck();
      return;
    }

    await pane.viewerCommands?.openSurfaceTab(
      surfaceId,
      target.label,
      target.path,
    );
    this.cdr.markForCheck();
  }

  get workspaceCycleCount(): number {
    return (
      (this.workspaceService.getWorkspaces().workspaces?.length ?? 0) +
      this.workspaceSurfaces.listToolWorkspaces().length
    );
  }

  cycleWorkspace(delta: -1 | 1): void {
    const files =
      this.workspaceService.getWorkspaces().workspaces ?? [];
    const tools = this.workspaceSurfaces.listToolWorkspaces();

    const sequence: Array<
      | { kind: 'files'; id: string }
      | { kind: 'tool'; id: string }
    > = [
      ...files.map(workspace => ({
        kind: 'files' as const,
        id: workspace.id,
      })),
      ...tools.map(workspace => ({
        kind: 'tool' as const,
        id: workspace.workspaceId,
      })),
    ];

    if (sequence.length < 2) return;

    const currentPresentation = this.workspacePresentation;
    const currentFiles = this.workspaceService.getActiveWorkspace();

    const currentIndex = sequence.findIndex(item =>
      currentPresentation?.kind === 'tool'
        ? item.kind === 'tool' &&
          item.id === currentPresentation.workspaceId
        : item.kind === 'files' &&
          item.id === currentFiles?.id,
    );

    const base = currentIndex >= 0 ? currentIndex : 0;
    const nextIndex =
      (base + delta + sequence.length) % sequence.length;
    const target = sequence[nextIndex];

    if (target.kind === 'files') {
      if (!this.workspaceService.switchWorkspace(target.id)) return;
      void this.applyActiveWorkspace();
      return;
    }

    if (!this.workspaceSurfaces.activate(target.id)) return;

    const presentation = this.workspaceSurfaces.get(target.id);
    if (!presentation) return;

    this.activateToolWorkspace(presentation);
  }

  async applyActiveWorkspace(): Promise<void> {
    const generation = ++this.workspaceApplyGeneration;
    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) return;

    const nextActivePane: PaneId =
      workspace.activePaneId === 'right' ? 'right' : 'left';

    this.activePaneId = nextActivePane;
    this.showHidden = !!workspace.showHidden;
    this.workspacePresentation =
      this.workspaceSurfaces.activateFilesWorkspace(workspace);

    this.leftPanePercent =
      typeof workspace.splitRatio === 'number' &&
      Number.isFinite(workspace.splitRatio) &&
      workspace.splitRatio > 0
        ? this.clampPanePercent(workspace.splitRatio)
        : 50;

    await Promise.all([
      this.leftPane?.tabCommands?.restoreWorkspaceState(),
      this.rightPane?.tabCommands?.restoreWorkspaceState(),
    ]);

    if (generation !== this.workspaceApplyGeneration) return;

    this.restorePaneIntelligenceForWorkspace(workspace.id);
    await this.ensurePaneIntelligenceHealth('workspace restore');

    // The workspace is the authority for active pane/layout; explicitly push
    // active-pane identity back through the service so all surfaces agree.
    this.workspaceService.setActivePaneId(nextActivePane);

    this.lastTransfer = 'none';
    this.lastDrop = 'none';
    this.closeContextMenu();
    this.closePaneIntelligenceMenu();

    queueMicrotask(() => this.activePane?.navigation?.focusContent());
    this.cdr.markForCheck();
  }

  async toggleHidden(): Promise<void> {
    const next = !this.showHidden;
    this.showHidden = next;

    const workspace = this.workspaceService.getActiveWorkspace();
    if (workspace) {
      this.workspaceService.updateWorkspace(workspace.id, {
        showHidden: next,
      });
    }

    const panes = [this.leftPane, this.rightPane]
      .filter((pane): pane is PaneHostComponent => !!pane);

    await Promise.all(
      panes
        .filter(pane => pane.contextState.isDirectoryTab)
        .map(pane => pane.navigation?.refresh()),
    );

    this.cdr.markForCheck();
  }

  beginResize(event: PointerEvent): void {
    if (typeof window !== 'undefined' && window.innerWidth <= 820) return;

    const target = event.currentTarget as HTMLElement | null;
    target?.setPointerCapture?.(event.pointerId);
    this.resizing = true;
    target?.classList.add('resizing');
    event.preventDefault();
  }

  resizeFromPointer(event: PointerEvent): void {
    if (!this.resizing) return;

    const root = this.managerRoot?.nativeElement;
    if (!root) return;

    const rect = root.getBoundingClientRect();
    if (rect.width <= 0) return;

    const percent = ((event.clientX - rect.left) / rect.width) * 100;
    this.leftPanePercent = this.clampPanePercent(percent);
    this.cdr.markForCheck();
  }

  endResize(event: PointerEvent): void {
    if (!this.resizing) return;

    const target = event.currentTarget as HTMLElement | null;
    if (target?.hasPointerCapture?.(event.pointerId)) {
      target.releasePointerCapture(event.pointerId);
    }

    this.resizing = false;
    target?.classList.remove('resizing');
    this.workspaceService.setSplitRatio(this.leftPanePercent);
    this.cdr.markForCheck();
  }

  resetSplit(): void {
    this.leftPanePercent = 50;
    this.workspaceService.setSplitRatio(50);
    this.cdr.markForCheck();
  }

  private restoreWorkspaceLayout(attempt = 0): void {
    const workspace = this.workspaceService.getActiveWorkspace();

    if (!workspace) {
      // WorkspaceService hydrates asynchronously. Bound the retry window so a
      // missing/corrupt workspace store cannot leave an immortal timer chain.
      if (attempt < 30) {
        setTimeout(() => this.restoreWorkspaceLayout(attempt + 1), 100);
      }
      return;
    }

    this.activePaneId =
      workspace.activePaneId === 'right' ? 'right' : 'left';
    this.showHidden = !!workspace.showHidden;
    this.workspacePresentation =
      this.workspaceSurfaces.activateFilesWorkspace(workspace);

    if (
      typeof workspace.splitRatio === 'number' &&
      Number.isFinite(workspace.splitRatio) &&
      workspace.splitRatio > 0
    ) {
      this.leftPanePercent = this.clampPanePercent(workspace.splitRatio);
    }

    this.cdr.markForCheck();
  }

  private syncFilesWorkspacePresentation(): void {
    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) return;

    this.workspacePresentation =
      this.workspaceSurfaces.activateFilesWorkspace(workspace);
    this.cdr.markForCheck();
  }

  private clampPanePercent(value: number): number {
    return Math.max(
      this.MIN_PANE_PERCENT,
      Math.min(this.MAX_PANE_PERCENT, value),
    );
  }

  private shortcutForCommand(commandId: string): string | undefined {
    const binding = BINDINGS.find(candidate =>
      candidate.commandId === commandId &&
      candidate.trigger.device === 'keyboard',
    );
    return binding ? this.formatKeyboardBinding(binding) : undefined;
  }

  private formatKeyboardBinding(
    binding: InputBindingDefinition,
  ): string {
    if (binding.trigger.device !== 'keyboard') return '';

    return binding.trigger.chord.steps
      .map(step => {
        const modifiers = (step.modifiers ?? []).map(modifier => {
          if (modifier === 'ctrl') return 'Ctrl';
          if (modifier === 'shift') return 'Shift';
          if (modifier === 'alt') return 'Alt';
          if (modifier === 'meta') return 'Meta';
          return modifier;
        });

        const names: Record<string, string> = {
          arrowup: 'в†‘',
          arrowdown: 'в†“',
          arrowleft: 'в†ђ',
          arrowright: 'в†’',
          escape: 'Esc',
          backspace: 'Backspace',
          delete: 'Delete',
          insert: 'Insert',
          pageup: 'PageUp',
          pagedown: 'PageDown',
          tab: 'Tab',
          enter: 'Enter',
          home: 'Home',
          end: 'End',
          ' ': 'Space',
        };

        const key = names[step.key] ?? step.key.toUpperCase();
        return [...modifiers, key].join('+');
      })
      .join(' then ');
  }

  private openCommandPaletteSurface(): void {
    const palette = this.commandPalette as (CommandPaletteComponent & { show?: () => void; open?: () => void }) | undefined;
    if (!palette) return;
    if (typeof palette.show === 'function') {
      palette.show();
      return;
    }
    palette.open?.();
  }

  private registerCommands(): void {
    const catalog: PaletteCommand[] = [];

    const descriptor = (
      id: string,
      title: string,
      category: string,
      execute: () => void | Promise<void>,
      enabled?: () => boolean,
    ): void => {
      this.input.registerCommandDescriptor({
        id,
        title,
        category,
        execute,
        isEnabled: enabled,
      }, OWNER_ID);

      catalog.push({
        id,
        title,
        category,
        execute,
        isEnabled: enabled,
        shortcut: this.shortcutForCommand(id),
      });
    };

    descriptor(COMMAND.toggleShortcutHelp, 'Input & Shortcuts', 'Help', () => this.toggleShortcutHelp());
    descriptor(COMMAND.showShortcutReference, 'Shortcut Reference', 'Help', () => this.showShortcutReference());
    descriptor(COMMAND.openCommandPalette, 'Command Palette', 'Navigation', async () => {
      await this.refreshCommandPaletteCommands();
      this.openCommandPaletteSurface();
    });
    descriptor(COMMAND.openSettings, 'Settings', 'Settings', () => this.openSettings());
    descriptor(COMMAND.openPluginSettings, 'Plugin Settings', 'Settings', () => this.openSettings('plugins'));
    descriptor(COMMAND.switchPane, 'Switch Pane', 'Pane Control', () => this.switchPane());
    descriptor(COMMAND.togglePaneLink, 'Toggle Linked Pane Navigation', 'Pane Intelligence', () => this.toggleLinkedNavigation(), () => !!this.leftPane?.contextState.isDirectoryTab && !!this.rightPane?.contextState.isDirectoryTab);
    descriptor(COMMAND.togglePaneMirror, 'Toggle Pane Presentation Mirroring', 'Pane Intelligence', () => this.toggleMirrorPresentation(), () => !!this.leftPane?.contextState.isDirectoryTab && !!this.rightPane?.contextState.isDirectoryTab);
    descriptor(COMMAND.togglePaneFilterMirror, 'Toggle Pane Filter Mirroring', 'Pane Intelligence', () => this.toggleMirrorFilter(), () => this.mirrorPresentation);
    descriptor(COMMAND.togglePaneScrollSync, 'Toggle Synchronized Pane Scrolling', 'Pane Intelligence', () => this.toggleSyncScroll(), () => !!this.leftPane?.contextState.isDirectoryTab && !!this.rightPane?.contextState.isDirectoryTab);
    descriptor(COMMAND.togglePaneFollow, 'Toggle Pane Follow Selection', 'Pane Intelligence', () => this.toggleFollowSelection(), () => !!this.leftPane?.contextState.isDirectoryTab && !!this.rightPane?.contextState.isDirectoryTab);
    descriptor(COMMAND.matchOtherPane, 'Match Other Pane to Active Location', 'Pane Intelligence', () => this.matchOtherPaneToActive(), () => this.canMatchOtherPane);
    descriptor(COMMAND.swapPaneRoots, 'Swap Linked Pane Roots', 'Pane Intelligence', () => this.swapLinkedRoots(), () => this.canSwapLinkedRoots);
    descriptor(COMMAND.openPeer, 'Open Current Peer Item', 'Pane Intelligence', () => this.openCurrentCounterpart(), () => this.canOpenCurrentCounterpart);
    descriptor(COMMAND.copyToPeer, 'Copy Current Item to Other Pane', 'Pane Intelligence', () => this.transferCurrentToPeer('copy'), () => this.canCopyCurrentToPeer);
    descriptor(COMMAND.moveToPeer, 'Move Current Item to Other Pane', 'Pane Intelligence', () => this.transferCurrentToPeer('move'), () => this.canMoveCurrentToPeer);
    descriptor(COMMAND.previousDifference, 'Previous Pane Difference', 'Pane Intelligence', () => this.navigateDifference(-1), () => this.differenceCount > 0);
    descriptor(COMMAND.nextDifference, 'Next Pane Difference', 'Pane Intelligence', () => this.navigateDifference(1), () => this.differenceCount > 0);
    descriptor(COMMAND.savePanePreset, 'Save Pane Pair Preset', 'Pane Intelligence', () => this.saveCurrentPanePreset(), () => !!this.leftPane?.contextState.path && !!this.rightPane?.contextState.path);
    descriptor(COMMAND.applyPanePreset, 'Apply Pane Pair Preset…', 'Pane Intelligence', () => this.chooseAndApplyPanePreset(), () => this.panePairPresets.length > 0);
    descriptor(COMMAND.widenLeftPane, 'Widen Left Pane', 'Layout', () => this.adjustSplit(5));
    descriptor(COMMAND.widenRightPane, 'Widen Right Pane', 'Layout', () => this.adjustSplit(-5));
    descriptor(COMMAND.resetPaneSplit, 'Reset Pane Split', 'Layout', () => this.resetSplit());
    descriptor(COMMAND.cursorUp, 'Move Cursor Up', 'Navigation', () => this.activePane?.selection?.moveCursor(-1));
    descriptor(COMMAND.cursorDown, 'Move Cursor Down', 'Navigation', () => this.activePane?.selection?.moveCursor(1));
    descriptor(COMMAND.open, 'Open Selected Item', 'Navigation', () => {
      const pane = this.activePane;
      const entry = pane?.selection.cursorEntry;
      if (pane && entry) void pane.viewerCommands?.openEntry(entry);
    });
    descriptor(COMMAND.up, 'Parent Directory', 'Navigation', () => this.activePane?.navigation?.navigateUp());
    descriptor(COMMAND.back, 'Back', 'Navigation', () => this.activePane?.navigation?.goBack());
    descriptor(COMMAND.forward, 'Forward', 'Navigation', () => this.activePane?.navigation?.goForward());
    descriptor(COMMAND.location, 'Edit Location', 'Navigation', () => this.activePane?.navigation?.focusLocation());
    descriptor(COMMAND.selectAll, 'Select All', 'Selection', () => this.activePane?.selection?.selectAll());
    descriptor(
      COMMAND.selectSameExtension,
      'Select Same Extension',
      'Selection',
      () => this.activePane?.selection?.selectSameExtension(),
      () => {
        const entry = this.activePane?.selection.cursorEntry;
        return !!entry && entry.type !== FileType.DIRECTORY;
      },
    );
    descriptor(
      COMMAND.selectSameType,
      'Select Same Type',
      'Selection',
      () => this.activePane?.selection?.selectSameType(),
      () => !!this.activePane?.selection.cursorEntry,
    );
    descriptor(COMMAND.clearSelection, 'Clear Selection', 'Selection', () => this.activePane?.selection?.clearSelection());
    descriptor(COMMAND.refresh, 'Refresh', 'Navigation', () => this.activePane?.navigation?.refresh());
    descriptor(COMMAND.rename, 'Rename', 'File Operations', () => this.activePane?.commands?.renameSelected(), () => !!this.activePane?.mutationState.canRename);
    descriptor(COMMAND.newFile, 'New File', 'File Operations', () => this.activePane?.commands?.newFile(), () => !!this.activePane?.mutationState.canCreate && !this.activePane?.contextState.isInArchive);
    descriptor(
      COMMAND.newTextDocument,
      'New Text Document',
      'File Operations',
      () => this.activePane?.commands?.newDocumentTemplate('text'),
      () => !!this.activePane?.mutationState.canCreate && !this.activePane?.contextState.isInArchive,
    );
    descriptor(
      COMMAND.newMarkdownDocument,
      'New Markdown Document',
      'File Operations',
      () => this.activePane?.commands?.newDocumentTemplate('markdown'),
      () => !!this.activePane?.mutationState.canCreate && !this.activePane?.contextState.isInArchive,
    );
    descriptor(
      COMMAND.newJsonDocument,
      'New JSON Document',
      'File Operations',
      () => this.activePane?.commands?.newDocumentTemplate('json'),
      () => !!this.activePane?.mutationState.canCreate && !this.activePane?.contextState.isInArchive,
    );
    descriptor(
      COMMAND.createShortcut,
      'Create Shortcut',
      'File Operations',
      () => this.activePane?.commands?.createShortcutForSelected(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!this.activePane?.mutationState.canCreate &&
        !!(
          this.activePane?.selection.cursorEntry ||
          this.activePane?.selection.selectedEntries.length
        ),
    );
    descriptor(COMMAND.properties, 'Properties', 'File Operations', () => this.activePane?.commands?.showSelectedProperties(), () => !!this.activePane?.selection.cursorEntry);
    descriptor(
      COMMAND.detailsWorkspace,
      'Open Full Details Workspace',
      'Workspace',
      () => this.openDetailsWorkspace(),
      () => !!this.activePane?.selection.cursorEntry,
    );
    descriptor(COMMAND.deletePermanent, 'Delete Permanently', 'File Operations', () => this.activePane?.commands?.deleteSelected(true), () => this.canDeleteActiveSelection());
    descriptor(COMMAND.newFolder, 'New Folder', 'File Operations', () => this.activePane?.commands?.newFolder(), () => !!this.activePane?.mutationState.canCreate);
    descriptor(
      COMMAND.moveSelectionToNewFolder,
      'Move Selection to New Folder',
      'File Operations',
      () => this.activePane?.commands?.moveSelectionToNewFolder(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!this.activePane?.mutationState.canCreate &&
        (this.activePane?.selection.selectedEntries.length ?? 0) > 0,
    );
    descriptor(COMMAND.deleteSelected, 'Delete', 'File Operations', () => this.deleteActiveSelection(), () => this.canDeleteActiveSelection());
    descriptor(COMMAND.copy, 'Copy', 'File Operations', () => this.activePane?.commands?.copySelected(), () => !!this.activePane?.mutationState.hasSelection);
    descriptor(COMMAND.cut, 'Cut', 'File Operations', () => this.activePane?.commands?.cutSelected(), () => !!this.activePane?.mutationState.canCut);
    descriptor(COMMAND.paste, 'Paste', 'File Operations', () => this.activePane?.commands?.paste(), () => !!this.activePane?.mutationState.canPaste);
    descriptor(
      COMMAND.copyPath,
      'Copy Path',
      'Files',
      () => this.activePane?.commands?.copySelectedPaths(),
      () => !!(
        this.activePane?.selection.cursorEntry ||
        this.activePane?.selection.selectedEntries.length
      ),
    );
    descriptor(
      COMMAND.copyRelativePath,
      'Copy Relative Path',
      'Files',
      () => this.activePane?.commands?.copySelectedRelativePaths(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !!(
          this.activePane?.selection.cursorEntry ||
          this.activePane?.selection.selectedEntries.length
        ),
    );
    descriptor(
      COMMAND.copyFileUri,
      'Copy File URI',
      'Files',
      () => this.activePane?.commands?.copySelectedFileUris(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!(
          this.activePane?.selection.cursorEntry ||
          this.activePane?.selection.selectedEntries.length
        ),
    );
    descriptor(
      COMMAND.revealInFileManager,
      'Reveal in System File Manager',
      'Files',
      () => this.activePane?.commands?.revealSelectedInFileManager(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!(
          this.activePane?.selection.cursorEntry ||
          this.activePane?.selection.selectedEntries.length
        ),
    );
    descriptor(
      COMMAND.setModifiedTime,
      'Set Modified Time…',
      'Files',
      () => this.activePane?.commands?.touchSelectedModifiedTime(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!(
          this.activePane?.selection.cursorEntry ||
          this.activePane?.selection.selectedEntries.length
        ),
    );
    descriptor(
      COMMAND.copyName,
      'Copy Name',
      'Files',
      () => this.activePane?.commands?.copySelectedNames(),
      () => !!(
        this.activePane?.selection.cursorEntry ||
        this.activePane?.selection.selectedEntries.length
      ),
    );
    descriptor(
      COMMAND.duplicate,
      'Duplicate',
      'File Operations',
      () => this.activePane?.commands?.duplicateSelected(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!this.activePane?.mutationState.canCreate &&
        (this.activePane?.selection.selectedEntries.length ?? 0) > 0,
    );
    descriptor(
      COMMAND.createSymlink,
      'Create Symbolic Link',
      'File Operations',
      () => this.activePane?.commands?.createLinkForSelected('symbolic'),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!this.activePane?.mutationState.canCreate &&
        !!(this.activePane?.selection.cursorEntry || this.activePane?.selection.selectedEntries.length),
    );
    descriptor(
      COMMAND.createHardlink,
      'Create Hard Link',
      'File Operations',
      () => this.activePane?.commands?.createLinkForSelected('hard'),
      () => {
        const entry =
          this.activePane?.selection.cursorEntry ??
          this.activePane?.selection.selectedEntries[0];
        return !!this.activePane?.contextState.isDirectoryTab &&
          !this.activePane?.contextState.isInArchive &&
          !!this.activePane?.mutationState.canCreate &&
          !!entry &&
          entry.type !== FileType.DIRECTORY;
      },
    );
    descriptor(
      COMMAND.openTerminal,
      'Open Terminal Here',
      'Navigation',
      () => this.openTerminalHere(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!this.activePane?.contextState.path,
    );
    descriptor(COMMAND.undo, 'Undo', 'Edit', () => this.undoLastOperation(), () => this.operationHistory.canUndo);
    descriptor(COMMAND.redo, 'Redo', 'Edit', () => this.redoLastOperation(), () => this.operationHistory.canRedo);
    descriptor(COMMAND.operationHistory, 'Operation History', 'Edit', () => { this.operationJournalVisible = true; this.cdr.markForCheck(); });
    descriptor(
      COMMAND.createArchive,
      'Create ZIP Archive',
      'Archive',
      () => this.activePane?.commands?.createZipFromSelection(),
      () => !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!this.activePane?.mutationState.canCreate &&
        (this.activePane?.selection.selectedEntries.length ?? 0) > 0,
    );
    descriptor(
      COMMAND.checksum,
      'SHA-256 Checksum',
      'Files',
      () => this.activePane?.commands?.showSelectedChecksum(),
      () => {
        const entry = this.activePane?.selection.cursorEntry ?? this.activePane?.selection.selectedEntries[0];
        return !!entry &&
          !this.activePane?.contextState.isInArchive &&
          entry.type !== FileType.DIRECTORY;
      },
    );
    descriptor(
      COMMAND.calculateSize,
      'Calculate Size',
      'Files',
      () => this.activePane?.selection?.calculateSelectedSize(),
      () => !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!(this.activePane?.selection.cursorEntry || this.activePane?.selection.selectedEntries.length),
    );
    descriptor(
      COMMAND.attributes,
      'Attributes / Permissions',
      'Files',
      () => this.activePane?.commands?.editSelectedAttributes(),
      () => !!this.activePane?.contextState.isDirectoryTab &&
        !this.activePane?.contextState.isInArchive &&
        !!(this.activePane?.selection.cursorEntry || this.activePane?.selection.selectedEntries.length),
    );
    descriptor(
      COMMAND.openTrash,
      'Trash / Recycle Bin',
      'Navigation',
      () => this.trashDialog.open(),
    );
    descriptor(COMMAND.copyToOther, 'Copy to Other Pane', 'Pane Control', () => this.copyToOther(), () => this.canTransfer('copy'));
    descriptor(COMMAND.moveToOther, 'Move to Other Pane', 'Pane Control', () => this.moveToOther(), () => this.canTransfer('move'));
    descriptor(COMMAND.newTab, 'New Tab', 'Tab Management', () => this.activePane?.tabCommands?.newTab());
    descriptor(COMMAND.closeTab, 'Close Tab', 'Tab Management', () => this.activePane?.tabCommands?.closeActiveTab());
    descriptor(
      COMMAND.reopenClosedTab,
      'Reopen Closed Tab',
      'Tab Management',
      () => this.reopenLastClosedTab(),
      () => this.workspaceService.canReopenClosedTab(),
    );
    descriptor(
      COMMAND.showClosedTabs,
      'Show Recently Closed Tabs',
      'Tab Management',
      () => {
        this.closedTabsVisible = true;
        this.cdr.markForCheck();
      },
      () => this.workspaceService.canReopenClosedTab(),
    );
    descriptor(COMMAND.nextTab, 'Next Tab', 'Tab Management', () => this.activePane?.tabCommands?.nextTab());
    descriptor(COMMAND.previousTab, 'Previous Tab', 'Tab Management', () => this.activePane?.tabCommands?.previousTab());
    descriptor(COMMAND.toggleTabPin, 'Pin/Unpin Tab', 'Tab Management', () => this.activePane?.tabCommands?.toggleActiveTabPin());
    descriptor(COMMAND.moveTabToOtherPane, 'Move Tab to Other Pane', 'Tab Management', () => this.moveActiveTabToOtherPane());
    descriptor(COMMAND.moveTabLeft, 'Move Tab Left', 'Tab Management', () => this.reorderActiveTab(-1));
    descriptor(COMMAND.moveTabRight, 'Move Tab Right', 'Tab Management', () => this.reorderActiveTab(1));
    descriptor(COMMAND.closeOtherTabs, 'Close Other Tabs', 'Tab Management', () => this.activePane?.tabCommands?.closeOtherTabs(), () => (this.activePane?.tabState.tabs.length ?? 0) > 1);
    descriptor(COMMAND.closeTabsToRight, 'Close Tabs to the Right', 'Tab Management', () => this.activePane?.tabCommands?.closeTabsToRight(), () => (this.activePane?.tabState.tabs.length ?? 0) > 1);
    descriptor(COMMAND.jumpTab1, 'Jump to Tab 1', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(0));
    descriptor(COMMAND.jumpTab2, 'Jump to Tab 2', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(1));
    descriptor(COMMAND.jumpTab3, 'Jump to Tab 3', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(2));
    descriptor(COMMAND.jumpTab4, 'Jump to Tab 4', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(3));
    descriptor(COMMAND.jumpTab5, 'Jump to Tab 5', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(4));
    descriptor(COMMAND.jumpTab6, 'Jump to Tab 6', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(5));
    descriptor(COMMAND.jumpTab7, 'Jump to Tab 7', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(6));
    descriptor(COMMAND.jumpTab8, 'Jump to Tab 8', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(7));
    descriptor(COMMAND.jumpTab9, 'Jump to Tab 9', 'Tab Management', () => this.activePane?.tabCommands?.jumpToTab(8));
    descriptor(COMMAND.viewerPrevious, 'Previous Viewer Item', 'Viewer', () => this.invokeViewer('showPrevious'), () => this.hasViewerMethod('showPrevious'));
    descriptor(COMMAND.viewerNext, 'Next Viewer Item', 'Viewer', () => this.invokeViewer('showNext'), () => this.hasViewerMethod('showNext'));
    descriptor(COMMAND.viewerZoomIn, 'Zoom Viewer In', 'Viewer', () => this.invokeViewer('zoomIn'), () => this.hasViewerMethod('zoomIn'));
    descriptor(COMMAND.viewerZoomOut, 'Zoom Viewer Out', 'Viewer', () => this.invokeViewer('zoomOut'), () => this.hasViewerMethod('zoomOut'));
    descriptor(COMMAND.viewerFit, 'Fit Viewer', 'Viewer', () => this.invokeViewer('resetZoom'), () => this.hasViewerMethod('resetZoom'));
    descriptor(COMMAND.viewerActualSize, 'Viewer Actual Size', 'Viewer', () => this.invokeViewer('zoom100'), () => this.hasViewerMethod('zoom100'));
    descriptor(COMMAND.viewerReturnSource, 'Return to Viewer Source Folder', 'Viewer', () => this.activePane?.viewerCommands?.returnToViewerSource(), () => !!this.activePane?.tabState.activeViewerTab?.sourceTabId || !!this.activePane?.tabState.activeViewerTab?.sourceDir);
    descriptor(COMMAND.viewerOpenExternal, 'Open Viewer File Externally', 'Viewer', () => this.activePane?.viewerCommands?.openActiveViewerExternally(), () => !!this.activePane?.tabState.activeViewerTab && !this.activePane.viewerState.viewerArchiveContext);
    descriptor(COMMAND.openAsText, 'Open as Text Viewer', 'Viewer', () => this.activePane?.viewerCommands?.openSelectedWithViewer('text'), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsBinary, 'Open as Binary Viewer', 'Viewer', () => this.activePane?.viewerCommands?.openSelectedWithViewer('binary'), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsImage, 'Open as Image Viewer', 'Viewer', () => this.activePane?.viewerCommands?.openSelectedWithViewer('image'), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsMarkdown, 'Open as Markdown Viewer', 'Viewer', () => this.activePane?.viewerCommands?.openSelectedWithViewer('markdown'), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsPdf, 'Open as PDF Viewer', 'Viewer', () => this.activePane?.viewerCommands?.openSelectedWithViewer('pdf'), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsLegacy, 'Open with Legacy Viewer', 'Viewer', () => this.activePane?.viewerCommands?.openSelectedWithViewer('legacy'), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.closeContextMenu, 'Close Context Menu', 'Navigation', () => this.closeContextMenu(), () => !!this.contextMenu);
    descriptor(COMMAND.focusFilter, 'Focus Folder Filter', 'Search & Filter', () => this.activePane?.navigation?.focusFilter(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.toggleHidden, 'Toggle Hidden Files', 'Workspace', () => this.toggleHidden());
    descriptor(COMMAND.showWorkspaceSwitcher, 'Show Workspace Switcher', 'Workspace', () => this.workspaceSwitcher?.open());
    descriptor(
      COMMAND.previousWorkspace,
      'Previous Workspace',
      'Workspace',
      () => this.cycleWorkspace(-1),
      () => this.workspaceCycleCount > 1,
    );
    descriptor(
      COMMAND.nextWorkspace,
      'Next Workspace',
      'Workspace',
      () => this.cycleWorkspace(1),
      () => this.workspaceCycleCount > 1,
    );
    descriptor(COMMAND.newWorkspace, 'New Workspace', 'Workspace', () => this.workspaceSwitcher?.createWorkspace());
    descriptor(COMMAND.showPathHistory, 'Show Path History', 'Navigation', () => this.openHistoryFor(this.activePaneId));
    descriptor(COMMAND.showQuickAccess, 'Open Quick Access', 'Navigation', () => this.openQuickAccessFor(this.activePaneId));
    descriptor(
      COMMAND.toggleCurrentPin,
      'Pin or Unpin Current Location',
      'Navigation',
      () => this.togglePanePin(this.activePane),
      () => !!this.activePane?.pinState.location,
    );
    descriptor(
      COMMAND.showPinnedLocations,
      'Open Pinned Locations',
      'Navigation',
      () => this.openQuickAccessFor(this.activePaneId, 'pins'),
    );
    descriptor(
      COMMAND.treeLeft,
      'Tree Collapse or Parent',
      'Navigation',
      () => this.activePane?.commands?.treeLeft(),
      () => this.activePane?.contextState.viewMode === 'tree',
    );
    descriptor(
      COMMAND.treeRight,
      'Tree Expand or Child',
      'Navigation',
      () => this.activePane?.commands?.treeRight(),
      () => this.activePane?.contextState.viewMode === 'tree',
    );
    descriptor(
      COMMAND.comparePanes,
      'Compare Panes',
      'Pane Control',
      () => this.togglePaneComparison(),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.compareWorkspace,
      'Open Compare Workspace',
      'Workspace',
      () => this.openCompareWorkspace(),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.recursiveComparePanes,
      'Recursive Pane Compare',
      'Pane Control',
      () => {
        this.comparisonVisible = true;
        if (!this.comparisonRecursive) {
          return this.toggleRecursiveComparison();
        }
        return this.refreshRecursiveComparison();
      },
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.reviewSyncPlan,
      'Review Sync Plan',
      'Pane Control',
      () => this.openSyncReview(),
      () => this.canComparePanes() && this.paneComparisonResult.summary.different > 0,
    );
    descriptor(
      COMMAND.selectLeftDifferences,
      'Select Left Differences',
      'Pane Control',
      () => this.selectComparisonDifferences('left'),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.selectRightDifferences,
      'Select Right Differences',
      'Pane Control',
      () => this.selectComparisonDifferences('right'),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.copyMissingLeftToRight,
      'Copy Missing Left to Right',
      'Pane Control',
      () => this.copyMissingComparison('left'),
      () => this.canComparePanes() && this.paneComparisonResult.summary.onlyLeft > 0,
    );
    descriptor(
      COMMAND.copyMissingRightToLeft,
      'Copy Missing Right to Left',
      'Pane Control',
      () => this.copyMissingComparison('right'),
      () => this.canComparePanes() && this.paneComparisonResult.summary.onlyRight > 0,
    );
    descriptor(
      COMMAND.refreshComparison,
      'Refresh Pane Comparison',
      'Pane Control',
      () => this.refreshComparison(),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.addLeftDifferencesToBasket,
      'Add Left Differences to Basket',
      'Pane Control',
      () => this.addComparisonDifferencesToBasket('left'),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.addRightDifferencesToBasket,
      'Add Right Differences to Basket',
      'Pane Control',
      () => this.addComparisonDifferencesToBasket('right'),
      () => this.canComparePanes(),
    );
    descriptor(
      COMMAND.addAllDifferencesToBasket,
      'Add All Differences to Basket',
      'Pane Control',
      () => this.addComparisonDifferencesToBasket('both'),
      () => this.canComparePanes(),
    );

    descriptor(COMMAND.toggleCurrentSelection, 'Toggle Current Selection', 'Selection', () => this.activePane?.selection?.toggleCurrentSelection(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.extendSelectionUp, 'Extend Selection Up', 'Selection', () => this.activePane?.selection?.extendSelection(-1), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.extendSelectionDown, 'Extend Selection Down', 'Selection', () => this.activePane?.selection?.extendSelection(1), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.selectByMask, 'Select by Mask', 'Selection', () => this.activePane?.selection?.selectByMaskPrompt(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.unselectByMask, 'Unselect by Mask', 'Selection', () => this.activePane?.selection?.unselectByMaskPrompt(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.invertSelection, 'Invert Selection', 'Selection', () => this.activePane?.selection?.invertSelection(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.selectSameExtension, 'Select Same Extension', 'Selection', () => this.activePane?.selection?.selectSameExtension(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.selectFiles, 'Select Visible Files', 'Selection', () => this.activePane?.selection?.selectFiles(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.selectFolders, 'Select Visible Folders', 'Selection', () => this.activePane?.selection?.selectFolders(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.unselectFiles, 'Unselect Visible Files', 'Selection', () => this.activePane?.selection?.unselectFiles(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.unselectFolders, 'Unselect Visible Folders', 'Selection', () => this.activePane?.selection?.unselectFolders(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.selectSameType, 'Select Same Type', 'Selection', () => this.activePane?.selection?.selectSameType(), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry);
    descriptor(COMMAND.cursorHome, 'Move Cursor to First', 'Navigation', () => this.activePane?.selection?.moveCursorHome(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.cursorEnd, 'Move Cursor to Last', 'Navigation', () => this.activePane?.selection?.moveCursorEnd(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.cursorPageUp, 'Page Up', 'Navigation', () => this.activePane?.selection?.moveCursorPage(-1), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.cursorPageDown, 'Page Down', 'Navigation', () => this.activePane?.selection?.moveCursorPage(1), () => !!this.activePane?.contextState.isDirectoryTab);

    descriptor(COMMAND.openBasket, 'Open Selection Basket', 'Tools', () => this.selectionBasket?.show());
    descriptor(COMMAND.addToBasket, 'Add to Selection Basket', 'Selection', () => this.addActiveSelectionToBasket(), () => !!this.activePane?.contextState.isDirectoryTab && (!!this.activePane?.mutationState.hasSelection || !!this.activePane?.selection.cursorPath));
    descriptor(COMMAND.clearBasket, 'Clear Selection Basket', 'Tools', () => this.selectionBasketService.clear(), () => this.selectionBasketService.snapshot.length > 0);

    descriptor(COMMAND.openOperations, 'Open Operations Center', 'Tools', () => this.openTools('operations'));
    descriptor(COMMAND.diskUsage, 'Disk Usage', 'Tools', () => this.openTools('disk'), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.findDuplicates, 'Find Duplicate Files', 'Tools', () => this.openTools('duplicates'), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.recentFiles, 'Recent Files', 'Tools', () => this.activePane?.viewerCommands?.openRecentFilesPanel(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(
      COMMAND.batchRename,
      'Batch Rename',
      'File Operations',
      () => this.batchRenameActiveSelection(),
      () =>
        !!this.activePane?.contextState.isDirectoryTab &&
        this.activePane.contextState.sourceCapabilities.rename,
    );
    descriptor(
      COMMAND.extractArchiveSelection,
      'Extract Archive Selection',
      'Archive',
      () => this.extractArchiveSelectionToFolder(),
      () => !!this.activePane?.contextState.isInArchive && !!this.activePane?.mutationState.hasSelection,
    );
    descriptor(
      COMMAND.extractArchiveToOther,
      'Extract Archive Selection to Other Pane',
      'Archive',
      () => this.extractArchiveSelectionToOther(),
      () =>
        !!this.activePane?.contextState.isInArchive &&
        !!this.activePane?.mutationState.hasSelection &&
        !!this.otherPane?.contextState.isDirectoryTab &&
        !!this.otherPane?.contextState.sourceCapabilities.create,
    );

    descriptor(COMMAND.globalSearch, 'Open Global Search', 'Search & Filter', () => this.openGlobalSearch(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.quickPathJump, 'Quick Path Jump', 'Navigation', () => this.quickPathJump(), () => !!this.activePane?.contextState.isDirectoryTab);
    descriptor(COMMAND.quickView, 'Quick View', 'File Operations', () => this.quickViewActive(), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openWithExternal, 'Open with External App', 'File Operations', () => this.openActiveExternally(), () => !!this.activePane?.contextState.isDirectoryTab && !!this.activePane?.selection.cursorEntry && this.activePane.selection.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(
      COMMAND.openWithChooser,
      'Open With…',
      'Files',
      () => this.openActiveWithChooser(),
      () => {
        const pane = this.activePane;
        if (!pane || pane.contextState.isInArchive) return false;
        if (pane.tabState.activeTab?.kind === 'viewer') {
          return !pane.tabState.activeViewerTab?.archiveContext;
        }
        return !!pane.contextState.isDirectoryTab &&
          !!pane.selection.cursorEntry &&
          pane.selection.cursorEntry.type !== FileType.DIRECTORY;
      },
    );
    descriptor(COMMAND.remoteConnect, 'Connect Remote Location', 'Navigation', () => this.connectRemote());
    this.commandPaletteCommands = catalog;
    this.cdr.markForCheck();
  }

  private activeViewer(): any {
    const pane = this.activePane;
    if (!pane || pane.tabState.activeTab?.kind !== 'viewer') return null;
    return pane.viewerState.mountedViewer;
  }

  private hasViewerMethod(method: string): boolean {
    const viewer = this.activeViewer();
    return !!viewer && typeof viewer[method] === 'function';
  }

  private invokeViewer(method: string): void {
    const viewer = this.activeViewer();
    const fn = viewer?.[method];
    if (typeof fn !== 'function') return;
    fn.call(viewer);
    this.cdr.markForCheck();
  }

  private canDeleteActiveSelection(): boolean {
    const pane = this.activePane;
    if (!pane) return false;
    if (pane.tabState.activeTab?.kind === 'viewer') return this.hasViewerMethod('deleteSelected');
    return pane.mutationState.canDelete;
  }

  private deleteActiveSelection(): void {
    const pane = this.activePane;
    if (!pane) return;
    if (pane.tabState.activeTab?.kind === 'viewer') {
      this.invokeViewer('deleteSelected');
      return;
    }
    void pane.commands?.deleteSelected();
  }

  private canTransfer(operation: 'copy' | 'move'): boolean {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination || !source.mutationState.hasSelection || !destination.contextState.path) {
      return false;
    }

    if (source.contextState.isInArchive) {
      if (!destination.contextState.sourceCapabilities.create) return false;
      if (operation === 'copy') return true;

      // ArchiveService.move can only truly remove the source from a top-level
      // archive. Nested archive moves are intentionally not presented as moves.
      const frame = source.contextState.currentArchiveFrame as { nestedChain?: readonly unknown[] } | null;
      return (frame?.nestedChain?.length ?? 1) === 0;
    }

    return canTransferBetween(
      operation,
      source.contextState.sourceCapabilities,
      destination.contextState.sourceCapabilities,
    ).allowed;
  }

  private async transferToOther(operation: 'copy' | 'move'): Promise<void> {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination) return;

    const selected = Array.from(source.selection?.selectedPaths ?? []) as string[];
    if (selected.length === 0) return;

    if (source.contextState.isInArchive) {
      if (!destination.contextState.sourceCapabilities.create) {
        await this.confirmDialog.alert({
          title: 'Archive Transfer',
          message: 'The destination pane is read-only.',
        });
        return;
      }

      const result = await source.commands?.transferArchiveSelectionTo(
        destination.contextState.path,
        operation,
      );
      if (!result) return;

      this.lastTransfer =
        `archive ${operation}: ${result.succeeded}/${result.total}`;

      await destination.navigation?.refresh();

      if (operation === 'move' && !result.sourceRemoved) {
        await this.confirmDialog.alert({
          title: 'Archive Move',
          message:
            'The entries were extracted to the destination, but the archive source ' +
            'could not be removed. The operation completed as a copy.',
        });
      }

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Archive Transfer Completed with Errors',
          message:
            `${result.failed} item${result.failed === 1 ? '' : 's'} could not be transferred.`,
        });
      }

      this.cdr.markForCheck();
      return;
    }

    const decision = canTransferBetween(operation, source.contextState.sourceCapabilities, destination.contextState.sourceCapabilities);
    if (!decision.allowed) {
      await this.confirmDialog.alert(transferBlockedMessage(operation, decision));
      return;
    }

    try {
      const items = await Promise.all(selected.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.api.joinPaths([destination.contextState.path, this.leafName(sourcePath)]),
      })));

      const queueId = this.powerFileOperations.enqueue(operation, items);
      const queued = await this.powerFileOperations.waitFor(queueId);

      if (queued.status !== 'completed' || !queued.result) {
        throw new Error(
          queued.error ||
          `${operation === 'copy' ? 'Copy' : 'Move'} did not complete.`,
        );
      }

      const result = queued.result;
      const affected = result.affectedPaths ?? [];
      if (
        result.failed === 0 &&
        result.skipped === 0 &&
        affected.length === result.succeeded &&
        affected.length > 0
      ) {
        const undoItems = affected.map(item => ({
          source: item.destination,
          destination: item.source,
          exactDestination: true,
        }));
        const redoItems = affected.map(item => ({
          source: item.source,
          destination: item.destination,
          exactDestination: true,
        }));

        this.operationHistory.record({
          label: `${operation === 'copy' ? 'Copy' : 'Move'} ${affected.length} item${
            affected.length === 1 ? '' : 's'
          }`,
          undo: async () => {
            if (operation === 'copy') {
              const deletion = await this.api.batchDelete(
                affected.map(item => item.destination),
                true,
                false,
              );
              if (deletion.failed > 0) {
                throw new Error('Undo copy failed.');
              }
            } else {
              const movedBack = await this.api.batchMove(undoItems, true);
              if (movedBack.failed > 0 || movedBack.skipped > 0) {
                throw new Error('Undo move failed.');
              }
            }
          },
          redo: async () => {
            const redone = operation === 'copy'
              ? await this.api.batchCopy(redoItems, true)
              : await this.api.batchMove(redoItems, true);
            if (redone.failed > 0 || redone.skipped > 0) {
              throw new Error(`Redo ${operation} failed.`);
            }
          },
        });
      }

      this.lastTransfer = `${operation}: ${result.succeeded}/${result.total}`;

      await destination.navigation?.refresh();
      if (operation === 'move') {
        await source.navigation?.refresh();
      }

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: `${operation === 'copy' ? 'Copy' : 'Move'} Completed with Errors`,
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be transferred.`,
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: `${operation === 'copy' ? 'Copy' : 'Move'} Error`,
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      this.cdr.markForCheck();
    }
  }

  private otherPaneId(): PaneId {
    return this.activePaneId === 'left' ? 'right' : 'left';
  }

  private leafName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(index + 1) : normalized;
  }
}