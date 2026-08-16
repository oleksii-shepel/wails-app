import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { ApiService } from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { WorkspaceService } from '../services/workspace.service';
import { PinnedLocationService } from '../services/pinned-location.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { canTransferBetween, transferBlockedMessage } from '../services/transfer-capabilities';
import { getSourceCapabilities } from '../services/source-capabilities';
import {
  PaneComparisonService,
  type PaneComparisonResult,
} from '../services/pane-comparison.service';
import { PaneRecursiveComparisonService } from '../services/pane-recursive-comparison.service';
import { SelectionBasketService } from '../services/selection-basket.service';
import {
  BatchRenameDialogService,
  type BatchRenameItem,
} from '../services/batch-rename-dialog.service';
import { RemoteConnectDialogService } from '../services/remote-connect-dialog.service';
import { ReassemblyDirectoryPaneComponent } from './reassembly-directory-pane.component';
import { ReassemblyWorkspaceSwitcherComponent } from './reassembly-workspace-switcher.component';
import { ReassemblyPathHistoryComponent } from './reassembly-path-history.component';
import { ReassemblyQuickAccessComponent } from './reassembly-quick-access.component';
import { ReassemblyRemoteConnectionsComponent } from './reassembly-remote-connections.component';
import { PaneComparisonBarComponent } from './pane-comparison-bar.component';
import {
  PaneSyncReviewComponent,
  type PaneSyncPlan,
} from './pane-sync-review.component';
import { SelectionBasketComponent } from './selection-basket.component';
import { ReassemblyToolsCenterComponent } from './reassembly-tools-center.component';
import { ReassemblyGlobalSearchComponent } from './reassembly-global-search.component';
import {
  ReassemblyCommandPaletteComponent,
  type ReassemblyPaletteCommand,
} from './reassembly-command-palette.component';
import {
  ReassemblyContextMenuComponent,
  type ReassemblyContextMenuItem,
  type ReassemblyContextMenuState,
} from './reassembly-context-menu.component';
import { FileInfo, FileType } from '../shared/protocol';

type PaneId = 'left' | 'right';

const CONTEXT_ID = 'reassembly.manager';
const OWNER_ID = 'reassembly.manager';

const COMMAND = {
  toggleShortcutHelp: 'reassembly.help.shortcuts',
  openCommandPalette: 'reassembly.navigation.commandPalette',
  switchPane: 'reassembly.manager.switchPane',
  widenLeftPane: 'reassembly.layout.widenLeft',
  widenRightPane: 'reassembly.layout.widenRight',
  resetPaneSplit: 'reassembly.layout.resetSplit',
  cursorUp: 'reassembly.manager.cursorUp',
  cursorDown: 'reassembly.manager.cursorDown',
  open: 'reassembly.manager.open',
  up: 'reassembly.manager.up',
  back: 'reassembly.manager.back',
  forward: 'reassembly.manager.forward',
  location: 'reassembly.manager.location',
  selectAll: 'reassembly.manager.selectAll',
  clearSelection: 'reassembly.manager.clearSelection',
  refresh: 'reassembly.manager.refresh',
  rename: 'reassembly.manager.rename',
  newFile: 'reassembly.manager.newFile',
  newFolder: 'reassembly.manager.newFolder',
  properties: 'reassembly.manager.properties',
  deletePermanent: 'reassembly.manager.deletePermanent',
  deleteSelected: 'reassembly.manager.delete',
  copy: 'reassembly.manager.copy',
  cut: 'reassembly.manager.cut',
  paste: 'reassembly.manager.paste',
  copyToOther: 'reassembly.manager.copyToOther',
  moveToOther: 'reassembly.manager.moveToOther',
  newTab: 'reassembly.manager.newTab',
  closeTab: 'reassembly.manager.closeTab',
  nextTab: 'reassembly.manager.nextTab',
  previousTab: 'reassembly.manager.previousTab',
  toggleTabPin: 'reassembly.manager.toggleTabPin',
  moveTabToOtherPane: 'reassembly.manager.moveTabToOtherPane',
  moveTabLeft: 'reassembly.manager.moveTabLeft',
  moveTabRight: 'reassembly.manager.moveTabRight',
  closeOtherTabs: 'reassembly.manager.closeOtherTabs',
  closeTabsToRight: 'reassembly.manager.closeTabsToRight',
  jumpTab1: 'reassembly.manager.jumpTab1',
  jumpTab2: 'reassembly.manager.jumpTab2',
  jumpTab3: 'reassembly.manager.jumpTab3',
  jumpTab4: 'reassembly.manager.jumpTab4',
  jumpTab5: 'reassembly.manager.jumpTab5',
  jumpTab6: 'reassembly.manager.jumpTab6',
  jumpTab7: 'reassembly.manager.jumpTab7',
  jumpTab8: 'reassembly.manager.jumpTab8',
  jumpTab9: 'reassembly.manager.jumpTab9',
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
  focusFilter: 'reassembly.manager.focusFilter',
  toggleHidden: 'reassembly.workspace.toggleHidden',
  showWorkspaceSwitcher: 'reassembly.workspace.showSwitcher',
  newWorkspace: 'reassembly.workspace.new',
  showPathHistory: 'reassembly.navigation.pathHistory',
  showQuickAccess: 'reassembly.navigation.quickAccess',
  toggleCurrentPin: 'reassembly.navigation.toggleCurrentPin',
  showPinnedLocations: 'reassembly.navigation.pinnedLocations',
  treeLeft: 'reassembly.tree.left',
  treeRight: 'reassembly.tree.right',
  comparePanes: 'reassembly.compare.toggle',
  recursiveComparePanes: 'reassembly.compare.recursive',
  reviewSyncPlan: 'reassembly.compare.review',
  selectLeftDifferences: 'reassembly.compare.selectLeftDifferences',
  selectRightDifferences: 'reassembly.compare.selectRightDifferences',
  copyMissingLeftToRight: 'reassembly.compare.copyMissingLeftToRight',
  copyMissingRightToLeft: 'reassembly.compare.copyMissingRightToLeft',
  refreshComparison: 'reassembly.compare.refresh',
  addLeftDifferencesToBasket: 'reassembly.compare.basketLeft',
  addRightDifferencesToBasket: 'reassembly.compare.basketRight',
  addAllDifferencesToBasket: 'reassembly.compare.basketAll',

  toggleCurrentSelection: 'reassembly.selection.toggleCurrent',
  extendSelectionUp: 'reassembly.selection.extendUp',
  extendSelectionDown: 'reassembly.selection.extendDown',
  selectByMask: 'reassembly.selection.selectByMask',
  unselectByMask: 'reassembly.selection.unselectByMask',
  invertSelection: 'reassembly.selection.invert',
  selectSameExtension: 'reassembly.selection.sameExtension',
  selectFiles: 'reassembly.selection.files',
  selectFolders: 'reassembly.selection.folders',
  unselectFiles: 'reassembly.selection.unselectFiles',
  unselectFolders: 'reassembly.selection.unselectFolders',
  selectSameType: 'reassembly.selection.sameType',
  cursorHome: 'reassembly.selection.cursorHome',
  cursorEnd: 'reassembly.selection.cursorEnd',
  cursorPageUp: 'reassembly.selection.cursorPageUp',
  cursorPageDown: 'reassembly.selection.cursorPageDown',

  openBasket: 'reassembly.basket.open',
  addToBasket: 'reassembly.basket.add',
  clearBasket: 'reassembly.basket.clear',

  openOperations: 'reassembly.tools.operations',
  diskUsage: 'reassembly.tools.diskUsage',
  findDuplicates: 'reassembly.tools.findDuplicates',
  batchRename: 'reassembly.files.batchRename',

  globalSearch: 'reassembly.navigation.globalSearch',
  quickPathJump: 'reassembly.navigation.quickPathJump',
  quickView: 'reassembly.files.quickView',
  openWithExternal: 'reassembly.files.openWithExternal',
  extractArchiveSelection: 'reassembly.archive.extractSelection',
  extractArchiveToOther: 'reassembly.archive.extractToOther',
  remoteConnect: 'reassembly.navigation.remoteConnect',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.layout.widenLeft.altShiftLeft', commandId:COMMAND.widenLeftPane, contextId:CONTEXT_ID, source:'default', description:'Widen left pane', category:'Layout', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft', modifiers:['alt','shift']}]}}},
  { id:'reassembly.layout.widenRight.altShiftRight', commandId:COMMAND.widenRightPane, contextId:CONTEXT_ID, source:'default', description:'Widen right pane', category:'Layout', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright', modifiers:['alt','shift']}]}}},
  { id:'reassembly.layout.resetSplit.altShift0', commandId:COMMAND.resetPaneSplit, contextId:CONTEXT_ID, source:'default', description:'Reset pane split', category:'Layout', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'0', modifiers:['alt','shift']}]}}},
  { id:'reassembly.help.shortcuts.f1', commandId:COMMAND.toggleShortcutHelp, contextId:CONTEXT_ID, source:'default', description:'Show keyboard shortcuts', category:'Help', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f1'}]}}},
  { id:'reassembly.files.deletePermanent.shiftDelete', commandId:COMMAND.deletePermanent, contextId:CONTEXT_ID, source:'default', description:'Delete permanently', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete', modifiers:['shift']}]}}},
  { id:'reassembly.files.properties.altEnter', commandId:COMMAND.properties, contextId:CONTEXT_ID, source:'default', description:'Properties', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'enter', modifiers:['alt']}]}}},
  { id:'reassembly.navigation.commandPalette.ctrlShiftP', commandId:COMMAND.openCommandPalette, contextId:CONTEXT_ID, source:'default', description:'Open command palette', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'p', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.manager.switchPane.tab', commandId:COMMAND.switchPane, contextId:CONTEXT_ID, source:'default', description:'Switch active pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'tab'}]}}},
  { id:'reassembly.manager.cursorUp.arrowUp', commandId:COMMAND.cursorUp, contextId:CONTEXT_ID, source:'default', description:'Move cursor up', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.manager.cursorDown.arrowDown', commandId:COMMAND.cursorDown, contextId:CONTEXT_ID, source:'default', description:'Move cursor down', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.manager.open.enter', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open selected item', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
  { id:'reassembly.manager.up.backspace', commandId:COMMAND.up, contextId:CONTEXT_ID, source:'default', description:'Parent directory', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'backspace'}]}}},
  { id:'reassembly.manager.back.altLeft', commandId:COMMAND.back, contextId:CONTEXT_ID, source:'default', description:'Back', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft', modifiers:['alt']}]}}},
  { id:'reassembly.manager.forward.altRight', commandId:COMMAND.forward, contextId:CONTEXT_ID, source:'default', description:'Forward', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright', modifiers:['alt']}]}}},
  { id:'reassembly.manager.location.ctrlL', commandId:COMMAND.location, contextId:CONTEXT_ID, source:'default', description:'Edit location', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'l', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.selectAll.ctrlA', commandId:COMMAND.selectAll, contextId:CONTEXT_ID, source:'default', description:'Select all', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'a', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.clearSelection.ctrlD', commandId:COMMAND.clearSelection, contextId:CONTEXT_ID, source:'default', description:'Clear selection', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'d', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.refresh.ctrlR', commandId:COMMAND.refresh, contextId:CONTEXT_ID, source:'default', description:'Refresh active pane', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'r', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.rename.f2', commandId:COMMAND.rename, contextId:CONTEXT_ID, source:'default', description:'Rename selected item', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f2'}]}}},
  { id:'reassembly.manager.newFolder.f7', commandId:COMMAND.newFolder, contextId:CONTEXT_ID, source:'default', description:'Create folder', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f7'}]}}},
  { id:'reassembly.manager.delete.delete', commandId:COMMAND.deleteSelected, contextId:CONTEXT_ID, source:'default', description:'Delete selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete'}]}}},
  { id:'reassembly.manager.delete.f8', commandId:COMMAND.deleteSelected, contextId:CONTEXT_ID, source:'default', description:'Delete selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f8'}]}}},
  { id:'reassembly.manager.copy.ctrlC', commandId:COMMAND.copy, contextId:CONTEXT_ID, source:'default', description:'Copy selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.cut.ctrlX', commandId:COMMAND.cut, contextId:CONTEXT_ID, source:'default', description:'Cut selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'x', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.paste.ctrlV', commandId:COMMAND.paste, contextId:CONTEXT_ID, source:'default', description:'Paste items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'v', modifiers:['ctrl']}]}}},
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
  { id:'reassembly.viewer.actualSize.ctrl1', commandId:COMMAND.viewerActualSize, contextId:CONTEXT_ID, source:'default', description:'Viewer actual size', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'1', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.contextMenu.escape', commandId:COMMAND.closeContextMenu, contextId:CONTEXT_ID, source:'default', description:'Close context menu', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}, preventDefault:true, stopPropagation:true },
  { id:'reassembly.manager.focusFilter.ctrlF', commandId:COMMAND.focusFilter, contextId:CONTEXT_ID, source:'default', description:'Focus folder filter', category:'Search & Filter', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f', modifiers:['ctrl']}]}}},
  { id:'reassembly.workspace.toggleHidden.ctrlShiftH', commandId:COMMAND.toggleHidden, contextId:CONTEXT_ID, source:'default', description:'Show/hide hidden files', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.workspace.showSwitcher.ctrlShiftW', commandId:COMMAND.showWorkspaceSwitcher, contextId:CONTEXT_ID, source:'default', description:'Open workspace switcher', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'w', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.workspace.new.ctrlShiftAltN', commandId:COMMAND.newWorkspace, contextId:CONTEXT_ID, source:'default', description:'Create workspace', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'n', modifiers:['ctrl','shift','alt']}]}}},
  { id:'reassembly.navigation.pathHistory.ctrlH', commandId:COMMAND.showPathHistory, contextId:CONTEXT_ID, source:'default', description:'Show path history', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.quickAccess.ctrlQ', commandId:COMMAND.showQuickAccess, contextId:CONTEXT_ID, source:'default', description:'Open Quick Access', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'q', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.pinnedLocations.ctrlB', commandId:COMMAND.showPinnedLocations, contextId:CONTEXT_ID, source:'default', description:'Open Quick Access pinned section', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'b', modifiers:['ctrl']}]}}},
  { id:'reassembly.tree.left.arrowLeft', commandId:COMMAND.treeLeft, contextId:CONTEXT_ID, source:'default', description:'Tree collapse or parent', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft'}]}}},
  { id:'reassembly.tree.right.arrowRight', commandId:COMMAND.treeRight, contextId:CONTEXT_ID, source:'default', description:'Tree expand or child', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright'}]}}},
  { id:'reassembly.compare.toggle.ctrlShiftC', commandId:COMMAND.comparePanes, contextId:CONTEXT_ID, source:'default', description:'Compare panes', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['ctrl','shift']}]}}},
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
  selector: 'app-reassembly-dual-pane',
  standalone: true,
  imports: [
    CommonModule,
    ReassemblyDirectoryPaneComponent,
    ReassemblyContextMenuComponent,
    ReassemblyWorkspaceSwitcherComponent,
    ReassemblyPathHistoryComponent,
    ReassemblyQuickAccessComponent,
    ReassemblyRemoteConnectionsComponent,
    PaneComparisonBarComponent,
    PaneSyncReviewComponent,
    SelectionBasketComponent,
    ReassemblyToolsCenterComponent,
    ReassemblyGlobalSearchComponent,

    ReassemblyCommandPaletteComponent,  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      #managerRoot
      class="manager"
      role="application"
      aria-label="Dual-pane file manager"
      aria-describedby="reassembly-manager-status"
      [style.grid-template-columns]="layoutColumns">
      <div id="reassembly-manager-status" class="sr-only" role="status" aria-live="polite">
        Active pane {{ activePaneId }}. Left pane {{ leftPanePercent | number:'1.0-0' }} percent.
      </div>
      <app-pane-comparison-bar
        class="comparison-surface"
        [visible]="comparisonVisible"
        [leftEntries]="leftPane.entries"
        [rightEntries]="rightPane.entries"
        [leftPath]="leftPane.path"
        [rightPath]="rightPane.path"
        [recursive]="comparisonRecursive"
        [scanning]="recursiveComparisonScanning"
        [error]="recursiveComparisonError"
        [comparisonResult]="paneComparisonResult"
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
        class="comparison-surface"
        [visible]="syncReviewVisible"
        [result]="paneComparisonResult"
        (execute)="executeSyncPlan($event)"
        (close)="closeSyncReview()">
      </app-pane-sync-review>
      <div class="pane-slot">
        <app-reassembly-directory-pane
          #leftPane
          paneId="left"
          [active]="activePaneId === 'left'"
          [showHidden]="showHidden"
          [currentPathPinned]="isPanePathPinned(leftPane)"
          (activateRequested)="activate('left')"
          (remoteRecoveryRequested)="openRemoteConnections()"
          (togglePinRequested)="togglePanePin(leftPane)"
          (quickAccessRequested)="openQuickAccessFor('left')"
          (historyRequested)="openHistoryFor('left')"
          (entryContextMenu)="openEntryMenu('left', $event.event, $event.entry)"
          (backgroundContextMenu)="openBackgroundMenu('left', $event)"
          (internalDrop)="handleInternalDrop('left', $event.event, $event.targetPath)">
        </app-reassembly-directory-pane>
      </div>

      <div
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

      <div class="pane-slot">
        <app-reassembly-directory-pane
          #rightPane
          paneId="right"
          [active]="activePaneId === 'right'"
          [showHidden]="showHidden"
          [currentPathPinned]="isPanePathPinned(rightPane)"
          (activateRequested)="activate('right')"
          (remoteRecoveryRequested)="openRemoteConnections()"
          (togglePinRequested)="togglePanePin(rightPane)"
          (quickAccessRequested)="openQuickAccessFor('right')"
          (historyRequested)="openHistoryFor('right')"
          (entryContextMenu)="openEntryMenu('right', $event.event, $event.entry)"
          (backgroundContextMenu)="openBackgroundMenu('right', $event)"
          (internalDrop)="handleInternalDrop('right', $event.event, $event.targetPath)">
        </app-reassembly-directory-pane>
      </div>

      <app-reassembly-command-palette
        #commandPalette
        [commands]="commandPaletteCommands">
      </app-reassembly-command-palette>

      <app-reassembly-global-search
        #globalSearch
        (openResult)="openSearchResult($event)"
        (openContainingFolder)="openSearchContainingFolder($event)">
      </app-reassembly-global-search>

      <app-reassembly-tools-center
        #toolsCenter
        (changed)="refreshBothPanes()"
        (navigateTo)="navigateFromTools($event)"
        (openBasketRequested)="selectionBasket?.show()">
      </app-reassembly-tools-center>

      <app-selection-basket
        #selectionBasket
        (copyTo)="copyBasketTo($event)"
        (moveTo)="moveBasketTo($event)"
        (deleteRequested)="deleteBasket()">
      </app-selection-basket>

      <app-reassembly-workspace-switcher
        #workspaceSwitcher
        (workspaceActivated)="applyActiveWorkspace()">
      </app-reassembly-workspace-switcher>

      <app-reassembly-path-history
        #pathHistory
        [paneId]="activePaneId"
        [currentPath]="activePane?.path ?? ''"
        (navigate)="navigateFromHistory($event)">
      </app-reassembly-path-history>

      <app-reassembly-quick-access
        #quickAccess
        [paneId]="activePaneId"
        [currentPath]="activePane?.path ?? ''"
        (navigate)="navigateFromQuickAccess($event)">
      </app-reassembly-quick-access>


      <app-reassembly-remote-connections
        #remoteConnections
        (navigate)="navigateFromRemoteConnections($event)">
      </app-reassembly-remote-connections>

      <app-reassembly-context-menu
        [state]="contextMenu"
        (command)="dispatchContextCommand($event)"
        (closed)="closeContextMenu()">
      </app-reassembly-context-menu>
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

    .comparison-surface {
      grid-column: 1 / -1;
      min-width: 0;
    }

    .pane-slot {
      grid-row: 3;
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
          var(--vsc-border, #3c3c3c) 40%,
          var(--vsc-border, #3c3c3c) 60%,
          transparent
        );
    }

    .divider:hover,
    .divider.resizing {
      background:
        linear-gradient(
          90deg,
          transparent,
          var(--vsc-accent-blue, #007acc) 35%,
          var(--vsc-accent-blue, #007acc) 65%,
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
            var(--vsc-border, #3c3c3c) 40%,
            var(--vsc-border, #3c3c3c) 60%,
            transparent
          );
      }

      .divider-grip {
        display: none;
      }
    }
    .divider:focus-visible{
      outline:2px solid var(--vsc-focus-border,#007fd4);
      outline-offset:-2px;
    }
    @container (max-width:760px){
      .divider{width:7px}
      .pane-slot{overflow:hidden}
    }
  `],
})
export class ReassemblyDualPaneComponent implements OnInit, OnDestroy {
  @ViewChild('managerRoot') managerRoot?: ElementRef<HTMLElement>;
  @ViewChild('workspaceSwitcher')
  workspaceSwitcher?: ReassemblyWorkspaceSwitcherComponent;
  @ViewChild('selectionBasket')
  selectionBasket?: SelectionBasketComponent;
  @ViewChild('toolsCenter')
  toolsCenter?: ReassemblyToolsCenterComponent;
  @ViewChild('commandPalette')
  commandPalette?: ReassemblyCommandPaletteComponent;
  @ViewChild('globalSearch')
  globalSearchOverlay?: ReassemblyGlobalSearchComponent;
  @ViewChild('pathHistory')
  pathHistory?: ReassemblyPathHistoryComponent;
  @ViewChild('quickAccess')
  quickAccess?: ReassemblyQuickAccessComponent;
  @ViewChild('leftPane') leftPane?: ReassemblyDirectoryPaneComponent;
  @ViewChild('rightPane') rightPane?: ReassemblyDirectoryPaneComponent;
  @ViewChild('remoteConnections') remoteConnections?: ReassemblyRemoteConnectionsComponent;

  activePaneId: PaneId = 'left';
  readonly MIN_PANE_PERCENT = 20;
  readonly MAX_PANE_PERCENT = 80;

  leftPanePercent = 50;
  resizing = false;
  private workspaceApplyGeneration = 0;
  showHidden = false;
  shortcutHelpVisible = false;

  commandPaletteCommands: ReassemblyPaletteCommand[] = [];

  comparisonVisible = false;
  syncReviewVisible = false;
  comparisonRecursive = false;
  recursiveComparisonScanning = false;
  recursiveComparisonError = '';
  recursiveComparisonResult: PaneComparisonResult | null = null;
  recursiveComparisonRoots: { left: string; right: string } | null = null;

  lastTransfer = 'none';
  lastDrop = 'none';
  contextMenu: ReassemblyContextMenuState | null = null;

  constructor(
    private readonly input: InputEngineService,
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly workspaceService: WorkspaceService,
    private readonly pinnedLocations: PinnedLocationService,
    private readonly paneComparison: PaneComparisonService,
    private readonly recursivePaneComparison: PaneRecursiveComparisonService,
    private readonly selectionBasketService: SelectionBasketService,
    private readonly batchRenameDialog: BatchRenameDialogService,
    private readonly remoteConnectDialog: RemoteConnectDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

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
    return BINDINGS
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

  get activePane(): ReassemblyDirectoryPaneComponent | undefined {
    return this.activePaneId === 'left' ? this.leftPane : this.rightPane;
  }

  get otherPane(): ReassemblyDirectoryPaneComponent | undefined {
    return this.activePaneId === 'left' ? this.rightPane : this.leftPane;
  }

  ngOnInit(): void {
    this.restoreWorkspaceLayout();

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
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const commandId of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(commandId, OWNER_ID);
    }
  }

  activate(paneId: PaneId, focusContent = false): void {
    this.closeContextMenu();

    if (this.activePaneId !== paneId) {
      this.activePaneId = paneId;
      this.workspaceService.setActivePaneId(paneId);
      this.cdr.markForCheck();
    }

    if (focusContent) {
      queueMicrotask(() => this.activePane?.focusContent());
    }
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
    const root = this.activePane?.isDirectoryTab
      ? this.activePane.path
      : this.otherPane?.isDirectoryTab
        ? this.otherPane.path
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
    if (!pane?.isDirectoryTab) return;

    const path = (await this.confirmDialog.prompt({
      title: 'Go to Path',
      message: 'Enter a local, VFS, or remote location.',
      confirmText: 'Go',
      defaultValue: pane.path,
      historyKey: 'reassembly.path.jump',
    }))?.trim();

    if (!path) return;
    pane.navigateTo(path);
  }

  quickViewActive(): void {
    const pane = this.activePane;
    const entry = pane?.cursorEntry;
    if (!pane?.isDirectoryTab || !entry) return;
    if (entry.type === FileType.DIRECTORY) return;
    pane.openQuickView(entry);
  }

  async openActiveExternally(): Promise<void> {
    const pane = this.activePane;
    if (!pane) return;

    if (pane.activeTab?.kind === 'viewer') {
      await pane.openActiveViewerExternally();
      return;
    }

    const entry = pane.cursorEntry;
    if (!pane.isDirectoryTab || !entry) return;
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

  openRemoteConnections(): void {
    this.remoteConnections?.open();
  }

  navigateFromRemoteConnections(path: string): void {
    const pane = this.activePane?.isDirectoryTab ? this.activePane :
      this.otherPane?.isDirectoryTab ? this.otherPane : null;
    if (!pane || !path) return;
    pane.navigateTo(path);
    pane.lastAction = `remote: ${path}`;
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
    if (!pane?.isDirectoryTab) return;

    if (entry.type === FileType.DIRECTORY) {
      pane.navigateTo(entry.path);
      return;
    }

    pane.openViewer(entry);
  }

  openSearchContainingFolder(entry: FileInfo): void {
    const pane = this.activePane;
    if (!pane?.isDirectoryTab) return;

    const target = entry.type === FileType.DIRECTORY
      ? entry.path
      : this.parentLocation(entry.path);

    pane.navigateTo(target);
    pane.lastAction = `search folder: ${target}`;
    this.cdr.markForCheck();
  }

  navigateFromTools(path: string): void {
    const pane = this.activePane?.isDirectoryTab
      ? this.activePane
      : this.otherPane?.isDirectoryTab
        ? this.otherPane
        : null;

    if (!pane || !path) return;
    pane.navigateTo(path);
    pane.lastAction = `tools: ${path}`;
    this.cdr.markForCheck();
  }

  openTools(page: 'operations' | 'disk' | 'duplicates'): void {
    const rootPath = this.activePane?.isDirectoryTab
      ? this.activePane.path
      : this.otherPane?.isDirectoryTab
        ? this.otherPane.path
        : '';

    this.toolsCenter?.open(page, rootPath);
  }

  async batchRenameActiveSelection(): Promise<void> {
    const pane = this.activePane;

    if (!pane?.isDirectoryTab) {
      await this.confirmDialog.alert({
        title: 'Batch Rename',
        message: 'Batch Rename is available from a directory pane.',
      });
      return;
    }

    if (!pane.sourceCapabilities.rename) {
      await this.confirmDialog.alert({
        title: 'Batch Rename',
        message: 'This location does not allow renaming.',
      });
      return;
    }

    const selected = pane.selectedEntries;
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

    const renamed = await this.batchRenameDialog.openDialog(items, pane.path);
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

  async moveActiveTabToOtherPane(): Promise<void> {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination) return;

    const tabId = source.exportActiveTabId();
    if (!tabId) return;

    const toIndex = destination.exportTabCount();
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
      source.reloadWorkspaceTabs(),
      destination.reloadWorkspaceTabs(tabId),
    ]);

    this.activate(destinationId);
    this.cdr.markForCheck();
  }

  async reorderActiveTab(delta: -1 | 1): Promise<void> {
    const pane = this.activePane;
    if (!pane) return;

    const tabId = pane.exportActiveTabId();
    if (!tabId) return;

    const currentCount = pane.exportTabCount();
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

    await pane.reloadWorkspaceTabs(tabId);
    this.cdr.markForCheck();
  }

  async refreshBothPanes(): Promise<void> {
    await Promise.all([
      this.leftPane?.refresh(),
      this.rightPane?.refresh(),
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
    if (!pane?.isDirectoryTab) return;

    const paths = pane.selectedPaths.size > 0
      ? Array.from(pane.selectedPaths)
      : pane.cursorPath
        ? [pane.cursorPath]
        : [];

    if (paths.length === 0) return;

    const added = this.selectionBasketService.add(
      paths,
      `pane:${this.activePaneId}`,
    );

    pane.lastAction = `basket: +${added}`;
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
        this.leftPane?.refresh(),
        this.rightPane?.refresh(),
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

    if (!destination?.isDirectoryTab || !destination.path) return;

    const paths = this.selectionBasketService.paths;
    if (paths.length === 0) return;

    if (!destination.sourceCapabilities.create) {
      await this.confirmDialog.alert({
        title: 'Basket Transfer',
        message: 'The destination pane is read-only.',
      });
      return;
    }

    for (const sourcePath of paths) {
      const decision = canTransferBetween(operation, getSourceCapabilities(sourcePath), destination.sourceCapabilities);
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
            destination.path,
            this.leafName(source),
          ]),
        })),
      );

      const result = operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      this.lastTransfer =
        `basket ${operation}: ${result.succeeded}/${result.total}`;

      await destination.refresh();

      // On a complete move, the basket no longer represents existing source
      // locations. Copies intentionally remain in the working set.
      if (operation === 'move' && result.failed === 0) {
        this.selectionBasketService.clear();
        await Promise.all([
          this.leftPane?.refresh(),
          this.rightPane?.refresh(),
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
      this.leftPane.isDirectoryTab &&
      this.rightPane.isDirectoryTab &&
      !!this.leftPane.path &&
      !!this.rightPane.path;
  }

  get paneComparisonResult(): PaneComparisonResult {
    const recursiveCurrent =
      this.comparisonRecursive &&
      !!this.recursiveComparisonResult &&
      !!this.recursiveComparisonRoots &&
      this.recursiveComparisonRoots.left === this.leftPane?.path &&
      this.recursiveComparisonRoots.right === this.rightPane?.path;

    if (recursiveCurrent) {
      return this.recursiveComparisonResult!;
    }

    return this.paneComparison.compare(
      this.leftPane?.entries ?? [],
      this.rightPane?.entries ?? [],
    );
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
        this.leftPane.path,
        this.rightPane.path,
        { showHidden: this.showHidden },
      );

      this.recursiveComparisonResult = result.comparison;
      this.recursiveComparisonRoots = {
        left: this.leftPane.path,
        right: this.rightPane.path,
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
      this.leftPane?.refresh(),
      this.rightPane?.refresh(),
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
    pane?.selectPaths(paths);
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
            destinationPane.path,
            relativePath,
          ]),
        };
      }),
    );

    await this.executeComparisonCopies(
      items,
      `Copy ${missing.length} missing item${missing.length === 1 ? '' : 's'} ${
        sourceSide === 'left' ? 'left → right' : 'right → left'
      }?`,
    );
  }

  async executeSyncPlan(plan: PaneSyncPlan): Promise<void> {
    if (!this.canComparePanes() || !plan) return;

    const approved = plan.items.filter(item => item.direction !== 'skip');
    if (approved.length === 0) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Execute Synchronization Plan',
      message:
        `Apply ${approved.length} approved synchronization action${
          approved.length === 1 ? '' : 's'
        }? Existing destination items will be overwritten.`,
      confirmText: 'Synchronize',
    });
    if (!confirmed) return;

    const items: Array<{ source: string; destination: string }> = [];

    for (const item of approved) {
      const leftToRight = item.direction === 'left-to-right';
      const source = leftToRight ? item.leftPath : item.rightPath;
      const destinationRoot = leftToRight
        ? this.rightPane?.path
        : this.leftPane?.path;

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
    this.lastTransfer = `sync: ${result.succeeded}/${result.total}`;

    this.syncReviewVisible = false;
    await Promise.all([
      this.leftPane?.refresh(),
      this.rightPane?.refresh(),
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
      this.leftPane?.refresh(),
      this.rightPane?.refresh(),
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
    if (!source?.isInArchive || !destination?.isDirectoryTab) return;

    const extracted = await source.extractSelectedTo(destination.path);
    if (!extracted) return;

    this.lastTransfer = 'archive extract to other pane';
    destination.lastAction = 'received extracted archive entries';
    await destination.refresh();
    this.cdr.markForCheck();
  }

  async extractArchiveSelectionToFolder(): Promise<void> {
    const source = this.activePane;
    if (!source?.isInArchive) return;

    const suggested = this.otherPane?.isDirectoryTab
      ? this.otherPane.path
      : '';

    const destination = (await this.confirmDialog.prompt({
      title: 'Extract Archive Entries',
      message: 'Extract selected archive entries to:',
      confirmText: 'Extract',
      defaultValue: suggested,
      historyKey: 'reassembly.archive.extract.destination',
    }))?.trim();

    if (!destination) return;

    const extracted = await source.extractSelectedTo(destination);
    if (!extracted) return;

    if (
      this.leftPane?.path === destination ||
      this.rightPane?.path === destination
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
    const operation: 'copy' | 'move' = event.shiftKey ? 'move' : 'copy';

    const sourceCapabilities = sourcePane?.sourceCapabilities ?? {
      read: true,
      create: false,
      delete: operation === 'move',
      rename: false,
      trash: false,
    };

    const decision = canTransferBetween(
      operation,
      sourceCapabilities,
      targetPane.sourceCapabilities,
    );

    if (!decision.allowed) {
      await this.confirmDialog.alert(
        transferBlockedMessage(operation, decision),
      );
      return;
    }

    const sourcePaths = payload.entries
      .map(entry => entry.path)
      .filter(path => !!path);

    if (sourcePaths.length === 0) return;

    // Dropping the same directory onto its own background is a no-op.
    if (
      payload.sourcePaneId === targetPaneId &&
      sourcePane?.path === targetPath
    ) {
      this.lastDrop = 'ignored: same directory';
      this.cdr.markForCheck();
      return;
    }

    try {
      if (sourcePane?.isInArchive) {
        const result = await sourcePane.transferArchiveSelectionTo(
          targetPath,
          operation,
        );
        if (!result) return;

        this.lastDrop =
          `archive ${operation} drop: ${result.succeeded}/${result.total}`;
        this.lastTransfer = this.lastDrop;
        targetPane.lastAction = this.lastDrop;

        await targetPane.refresh();

        if (operation === 'move' && result.sourceRemoved) {
          await sourcePane.refresh();
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

      this.lastDrop = `${operation} drop: ${result.succeeded}/${result.total}`;
      this.lastTransfer = this.lastDrop;
      targetPane.lastAction = this.lastDrop;

      await targetPane.refresh();

      if (
        operation === 'move' &&
        sourcePane &&
        sourcePane !== targetPane
      ) {
        await sourcePane.refresh();
      } else if (
        operation === 'move' &&
        sourcePane &&
        sourcePane === targetPane &&
        targetPath !== sourcePane.path
      ) {
        await sourcePane.refresh();
      }

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Drop Completed with Errors',
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be ${operation === 'copy' ? 'copied' : 'moved'}.`,
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
    entries: Array<{ path: string; name?: string; type?: unknown }>;
  } | null {
    try {
      const raw = event.dataTransfer?.getData('application/json');
      if (!raw) return null;

      const parsed = JSON.parse(raw) as {
        sourcePaneId?: unknown;
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

      return { sourcePaneId, entries };
    } catch {
      return null;
    }
  }

  openEntryMenu(
    paneId: PaneId,
    event: MouseEvent,
    entry: FileInfo,
  ): void {
    this.activePaneId = paneId;

    const items: ReassemblyContextMenuItem[] = [
      this.menuCommand(COMMAND.open, 'Open', 'Enter'),
      this.menuCommand(COMMAND.quickView, 'Quick view', 'F3'),
      this.menuCommand(COMMAND.openWithExternal, 'Open externally', 'Ctrl+Shift+O'),
      { separator: true },
      this.menuCommand(COMMAND.rename, 'Rename', 'F2'),
      this.menuCommand(COMMAND.selectSameExtension, 'Select same extension', 'Alt+E'),
      this.menuCommand(COMMAND.selectSameType, 'Select same type'),
      this.menuCommand(COMMAND.properties, 'Properties', 'Alt+Enter'),
      this.menuCommand(COMMAND.batchRename, 'Batch rename…', 'Ctrl+Shift+R'),
      this.menuCommand(COMMAND.copy, 'Copy', 'Ctrl+C'),
      this.menuCommand(COMMAND.cut, 'Cut', 'Ctrl+X'),
      this.menuCommand(COMMAND.paste, 'Paste', 'Ctrl+V'),
      { separator: true },
      this.menuCommand(COMMAND.copyToOther, 'Copy to other pane', 'F5'),
      this.menuCommand(COMMAND.moveToOther, 'Move to other pane', 'F6'),
      this.menuCommand(COMMAND.addToBasket, 'Add to Selection Basket', 'Ctrl+Alt+B'),
      this.menuCommand(COMMAND.extractArchiveToOther, 'Extract to other pane'),
      this.menuCommand(COMMAND.extractArchiveSelection, 'Extract to folder…'),
      { separator: true },
      this.menuCommand(COMMAND.deleteSelected, 'Delete', 'Delete'),
      this.menuCommand(COMMAND.deletePermanent, 'Delete permanently', 'Shift+Delete'),
    ];

    this.contextMenu = {
      x: this.clampMenuX(event.clientX),
      y: this.clampMenuY(event.clientY),
      title: entry.name,
      items,
    };
    this.cdr.markForCheck();
  }

  openBackgroundMenu(paneId: PaneId, event: MouseEvent): void {
    this.activePaneId = paneId;

    this.contextMenu = {
      x: this.clampMenuX(event.clientX),
      y: this.clampMenuY(event.clientY),
      title: this.activePane?.path || 'Folder',
      items: [
        this.menuCommand(COMMAND.newFolder, 'New folder', 'F7'),
        this.menuCommand(
          COMMAND.toggleCurrentPin,
          this.isPanePathPinned(this.activePane) ? 'Unpin this location' : 'Pin this location',
        ),
        this.menuCommand(COMMAND.paste, 'Paste', 'Ctrl+V'),
        { separator: true },
        this.menuCommand(COMMAND.selectAll, 'Select all', 'Ctrl+A'),
        this.menuCommand(COMMAND.invertSelection, 'Invert visible selection', 'Alt+I'),
        this.menuCommand(COMMAND.selectFiles, 'Select visible files', 'Alt+Shift+F'),
        this.menuCommand(COMMAND.selectFolders, 'Select visible folders', 'Alt+Shift+D'),
        this.menuCommand(COMMAND.clearSelection, 'Clear selection', 'Ctrl+D'),
        this.menuCommand(COMMAND.invertSelection, 'Invert selection', 'Alt+I'),
        this.menuCommand(COMMAND.selectByMask, 'Select by mask…', 'Alt+S'),
        this.menuCommand(COMMAND.unselectByMask, 'Unselect by mask…', 'Alt+U'),
        this.menuCommand(COMMAND.openBasket, 'Selection Basket', 'Ctrl+Shift+B'),
        this.menuCommand(COMMAND.globalSearch, 'Search here…', 'Ctrl+K'),
        this.menuCommand(COMMAND.quickPathJump, 'Go to path…', 'Ctrl+P'),
        this.menuCommand(COMMAND.remoteConnect, 'Connect remote…'),
        this.menuCommand(COMMAND.diskUsage, 'Disk Usage', 'Ctrl+Shift+D'),
        this.menuCommand(COMMAND.findDuplicates, 'Find Duplicates'),
        this.menuCommand(COMMAND.openCommandPalette, 'Command Palette', 'Ctrl+Shift+P'),
        this.menuCommand(COMMAND.openOperations, 'Operations Center'),
        { separator: true },
        this.menuCommand(COMMAND.toggleTabPin, 'Pin / unpin current tab', 'Ctrl+Alt+P'),
        this.menuCommand(COMMAND.moveTabToOtherPane, 'Move current tab to other pane'),
        this.menuCommand(COMMAND.moveTabLeft, 'Move tab left'),
        this.menuCommand(COMMAND.moveTabRight, 'Move tab right'),
        this.menuCommand(COMMAND.closeOtherTabs, 'Close other tabs'),
        this.menuCommand(COMMAND.closeTabsToRight, 'Close tabs to the right'),
        this.menuCommand(COMMAND.newFile, 'New File'),
        this.menuCommand(COMMAND.refresh, 'Refresh', 'Ctrl+R'),
      ],
    };
    this.cdr.markForCheck();
  }

  dispatchContextCommand(commandId: string): void {
    this.closeContextMenu();
    this.input.dispatchCommandById(commandId, {
      source: 'context-menu',
      context: {
        paneId: this.activePaneId,
        path: this.activePane?.path ?? '',
      },
    });
  }

  closeContextMenu(): void {
    if (!this.contextMenu) return;
    this.contextMenu = null;
    this.cdr.markForCheck();
  }

  private menuCommand(
    commandId: string,
    label: string,
    shortcut?: string,
  ): ReassemblyContextMenuItem {
    const state = this.input.getCommandState(commandId);
    return {
      commandId,
      label,
      shortcut,
      enabled: !!state?.registered && state.enabled,
    };
  }

  private clampMenuX(x: number): number {
    if (typeof window === 'undefined') return x;
    return Math.max(8, Math.min(x, window.innerWidth - 258));
  }

  private clampMenuY(y: number): number {
    if (typeof window === 'undefined') return y;
    return Math.max(8, Math.min(y, window.innerHeight - 360));
  }

  isPanePathPinned(
    pane: ReassemblyDirectoryPaneComponent | undefined,
  ): boolean {
    if (!pane?.isDirectoryTab || !pane.path) return false;
    return this.pinnedLocations.isPinned(
      this.pinnedLocations.load(),
      pane.path,
      'dir',
    );
  }

  togglePanePin(
    pane: ReassemblyDirectoryPaneComponent | undefined,
  ): void {
    if (!pane?.isDirectoryTab || !pane.path) return;

    let pins = this.pinnedLocations.load();
    const pinned = this.pinnedLocations.isPinned(pins, pane.path, 'dir');

    if (pinned) {
      pins = this.pinnedLocations.remove(pins, pane.path, 'dir');
      pane.lastAction = `unpin: ${pane.path}`;
    } else {
      const pin = this.pinnedLocations.create(
        pane.path,
        undefined,
        undefined,
        Date.now(),
        'dir',
      );
      if (!pin) return;
      pins = [...pins.filter(existing => existing.id !== pin.id), pin];
      pane.lastAction = `pin: ${pane.path}`;
    }

    this.pinnedLocations.save(pins);
    this.cdr.markForCheck();
  }


  navigateFromHistory(path: string): void {
    const pane = this.activePane;
    if (!pane?.isDirectoryTab) return;

    pane.navigateTo(path);
    pane.lastAction = `history: ${path}`;
    this.cdr.markForCheck();
  }

  navigateFromQuickAccess(path: string): void {
    const pane = this.activePane;
    if (!pane?.isDirectoryTab) return;

    pane.navigateTo(path);
    pane.lastAction = `quick access: ${path}`;
    this.cdr.markForCheck();
  }

  async applyActiveWorkspace(): Promise<void> {
    const generation = ++this.workspaceApplyGeneration;
    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) return;

    const nextActivePane: PaneId =
      workspace.activePaneId === 'right' ? 'right' : 'left';

    this.activePaneId = nextActivePane;
    this.showHidden = !!workspace.showHidden;

    this.leftPanePercent =
      typeof workspace.splitRatio === 'number' &&
      Number.isFinite(workspace.splitRatio) &&
      workspace.splitRatio > 0
        ? this.clampPanePercent(workspace.splitRatio)
        : 50;

    await Promise.all([
      this.leftPane?.restoreWorkspaceState(),
      this.rightPane?.restoreWorkspaceState(),
    ]);

    if (generation !== this.workspaceApplyGeneration) return;

    // The workspace is the authority for active pane/layout; explicitly push
    // active-pane identity back through the service so all surfaces agree.
    this.workspaceService.setActivePaneId(nextActivePane);

    this.lastTransfer = 'none';
    this.lastDrop = 'none';
    this.closeContextMenu();

    queueMicrotask(() => this.activePane?.focusContent());
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
      .filter((pane): pane is ReassemblyDirectoryPaneComponent => !!pane);

    await Promise.all(
      panes
        .filter(pane => pane.isDirectoryTab)
        .map(pane => pane.refresh()),
    );

    for (const pane of panes) {
      pane.lastAction = next ? 'show hidden' : 'hide hidden';
    }

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

    if (
      typeof workspace.splitRatio === 'number' &&
      Number.isFinite(workspace.splitRatio) &&
      workspace.splitRatio > 0
    ) {
      this.leftPanePercent = this.clampPanePercent(workspace.splitRatio);
    }

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
          arrowup: '↑',
          arrowdown: '↓',
          arrowleft: '←',
          arrowright: '→',
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

  private registerCommands(): void {
    const catalog: ReassemblyPaletteCommand[] = [];

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

    descriptor(COMMAND.toggleShortcutHelp, 'Keyboard Shortcuts', 'Help', () => this.toggleShortcutHelp());
    descriptor(COMMAND.openCommandPalette, 'Command Palette', 'Navigation', () => this.commandPalette?.open());
    descriptor(COMMAND.switchPane, 'Switch Pane', 'Pane Control', () => this.switchPane());
    descriptor(COMMAND.widenLeftPane, 'Widen Left Pane', 'Layout', () => this.adjustSplit(5));
    descriptor(COMMAND.widenRightPane, 'Widen Right Pane', 'Layout', () => this.adjustSplit(-5));
    descriptor(COMMAND.resetPaneSplit, 'Reset Pane Split', 'Layout', () => this.resetSplit());
    descriptor(COMMAND.cursorUp, 'Move Cursor Up', 'Navigation', () => this.activePane?.moveCursor(-1));
    descriptor(COMMAND.cursorDown, 'Move Cursor Down', 'Navigation', () => this.activePane?.moveCursor(1));
    descriptor(COMMAND.open, 'Open Selected Item', 'Navigation', () => {
      const pane = this.activePane;
      const entry = pane?.cursorEntry;
      if (pane && entry) pane.openEntry(entry);
    });
    descriptor(COMMAND.up, 'Parent Directory', 'Navigation', () => this.activePane?.navigateUp());
    descriptor(COMMAND.back, 'Back', 'Navigation', () => this.activePane?.goBack());
    descriptor(COMMAND.forward, 'Forward', 'Navigation', () => this.activePane?.goForward());
    descriptor(COMMAND.location, 'Edit Location', 'Navigation', () => this.activePane?.focusLocation());
    descriptor(COMMAND.selectAll, 'Select All', 'Selection', () => this.activePane?.selectAll());
    descriptor(COMMAND.clearSelection, 'Clear Selection', 'Selection', () => this.activePane?.clearSelection());
    descriptor(COMMAND.refresh, 'Refresh', 'Navigation', () => this.activePane?.refresh());
    descriptor(COMMAND.rename, 'Rename', 'File Operations', () => this.activePane?.renameSelected(), () => !!this.activePane?.canRename);
    descriptor(COMMAND.newFile, 'New File', 'File Operations', () => this.activePane?.newFile(), () => !!this.activePane?.canCreate && !this.activePane?.isInArchive);
    descriptor(COMMAND.properties, 'Properties', 'File Operations', () => this.activePane?.showSelectedProperties(), () => !!this.activePane?.cursorEntry);
    descriptor(COMMAND.deletePermanent, 'Delete Permanently', 'File Operations', () => this.activePane?.deleteSelected(true), () => this.canDeleteActiveSelection());
    descriptor(COMMAND.newFolder, 'New Folder', 'File Operations', () => this.activePane?.newFolder(), () => !!this.activePane?.canCreate);
    descriptor(COMMAND.deleteSelected, 'Delete', 'File Operations', () => this.deleteActiveSelection(), () => this.canDeleteActiveSelection());
    descriptor(COMMAND.copy, 'Copy', 'File Operations', () => this.activePane?.copySelected(), () => !!this.activePane?.hasSelection);
    descriptor(COMMAND.cut, 'Cut', 'File Operations', () => this.activePane?.cutSelected(), () => !!this.activePane?.canCut);
    descriptor(COMMAND.paste, 'Paste', 'File Operations', () => this.activePane?.paste(), () => !!this.activePane?.canPaste);
    descriptor(COMMAND.copyToOther, 'Copy to Other Pane', 'Pane Control', () => this.copyToOther(), () => this.canTransfer('copy'));
    descriptor(COMMAND.moveToOther, 'Move to Other Pane', 'Pane Control', () => this.moveToOther(), () => this.canTransfer('move'));
    descriptor(COMMAND.newTab, 'New Tab', 'Tab Management', () => this.activePane?.newTab());
    descriptor(COMMAND.closeTab, 'Close Tab', 'Tab Management', () => this.activePane?.closeActiveTab());
    descriptor(COMMAND.nextTab, 'Next Tab', 'Tab Management', () => this.activePane?.nextTab());
    descriptor(COMMAND.previousTab, 'Previous Tab', 'Tab Management', () => this.activePane?.previousTab());
    descriptor(COMMAND.toggleTabPin, 'Pin/Unpin Tab', 'Tab Management', () => this.activePane?.toggleActiveTabPin());
    descriptor(COMMAND.moveTabToOtherPane, 'Move Tab to Other Pane', 'Tab Management', () => this.moveActiveTabToOtherPane());
    descriptor(COMMAND.moveTabLeft, 'Move Tab Left', 'Tab Management', () => this.reorderActiveTab(-1));
    descriptor(COMMAND.moveTabRight, 'Move Tab Right', 'Tab Management', () => this.reorderActiveTab(1));
    descriptor(COMMAND.closeOtherTabs, 'Close Other Tabs', 'Tab Management', () => this.activePane?.closeOtherTabs(), () => (this.activePane?.tabs.length ?? 0) > 1);
    descriptor(COMMAND.closeTabsToRight, 'Close Tabs to the Right', 'Tab Management', () => this.activePane?.closeTabsToRight(), () => (this.activePane?.tabs.length ?? 0) > 1);
    descriptor(COMMAND.jumpTab1, 'Jump to Tab 1', 'Tab Management', () => this.activePane?.jumpToTab(0));
    descriptor(COMMAND.jumpTab2, 'Jump to Tab 2', 'Tab Management', () => this.activePane?.jumpToTab(1));
    descriptor(COMMAND.jumpTab3, 'Jump to Tab 3', 'Tab Management', () => this.activePane?.jumpToTab(2));
    descriptor(COMMAND.jumpTab4, 'Jump to Tab 4', 'Tab Management', () => this.activePane?.jumpToTab(3));
    descriptor(COMMAND.jumpTab5, 'Jump to Tab 5', 'Tab Management', () => this.activePane?.jumpToTab(4));
    descriptor(COMMAND.jumpTab6, 'Jump to Tab 6', 'Tab Management', () => this.activePane?.jumpToTab(5));
    descriptor(COMMAND.jumpTab7, 'Jump to Tab 7', 'Tab Management', () => this.activePane?.jumpToTab(6));
    descriptor(COMMAND.jumpTab8, 'Jump to Tab 8', 'Tab Management', () => this.activePane?.jumpToTab(7));
    descriptor(COMMAND.jumpTab9, 'Jump to Tab 9', 'Tab Management', () => this.activePane?.jumpToTab(8));
    descriptor(COMMAND.viewerPrevious, 'Previous Viewer Item', 'Viewer', () => this.invokeViewer('showPrevious'), () => this.hasViewerMethod('showPrevious'));
    descriptor(COMMAND.viewerNext, 'Next Viewer Item', 'Viewer', () => this.invokeViewer('showNext'), () => this.hasViewerMethod('showNext'));
    descriptor(COMMAND.viewerZoomIn, 'Zoom Viewer In', 'Viewer', () => this.invokeViewer('zoomIn'), () => this.hasViewerMethod('zoomIn'));
    descriptor(COMMAND.viewerZoomOut, 'Zoom Viewer Out', 'Viewer', () => this.invokeViewer('zoomOut'), () => this.hasViewerMethod('zoomOut'));
    descriptor(COMMAND.viewerFit, 'Fit Viewer', 'Viewer', () => this.invokeViewer('resetZoom'), () => this.hasViewerMethod('resetZoom'));
    descriptor(COMMAND.viewerActualSize, 'Viewer Actual Size', 'Viewer', () => this.invokeViewer('zoom100'), () => this.hasViewerMethod('zoom100'));
    descriptor(COMMAND.viewerReturnSource, 'Return to Viewer Source Folder', 'Viewer', () => this.activePane?.returnToViewerSource(), () => !!this.activePane?.activeViewerTab?.sourceTabId || !!this.activePane?.activeViewerTab?.sourceDir);
    descriptor(COMMAND.viewerOpenExternal, 'Open Viewer File Externally', 'Viewer', () => this.activePane?.openActiveViewerExternally(), () => !!this.activePane?.activeViewerTab && !this.activePane.viewerArchiveContext);
    descriptor(COMMAND.openAsText, 'Open as Text Viewer', 'Viewer', () => this.activePane?.openSelectedWithViewer('text'), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsBinary, 'Open as Binary Viewer', 'Viewer', () => this.activePane?.openSelectedWithViewer('binary'), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsImage, 'Open as Image Viewer', 'Viewer', () => this.activePane?.openSelectedWithViewer('image'), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsMarkdown, 'Open as Markdown Viewer', 'Viewer', () => this.activePane?.openSelectedWithViewer('markdown'), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsPdf, 'Open as PDF Viewer', 'Viewer', () => this.activePane?.openSelectedWithViewer('pdf'), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openAsLegacy, 'Open with Legacy Viewer', 'Viewer', () => this.activePane?.openSelectedWithViewer('legacy'), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.closeContextMenu, 'Close Context Menu', 'Navigation', () => this.closeContextMenu(), () => !!this.contextMenu);
    descriptor(COMMAND.focusFilter, 'Focus Folder Filter', 'Search & Filter', () => this.activePane?.focusFilter(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.toggleHidden, 'Toggle Hidden Files', 'Workspace', () => this.toggleHidden());
    descriptor(COMMAND.showWorkspaceSwitcher, 'Show Workspace Switcher', 'Workspace', () => this.workspaceSwitcher?.open());
    descriptor(COMMAND.newWorkspace, 'New Workspace', 'Workspace', () => this.workspaceSwitcher?.createWorkspace());
    descriptor(COMMAND.showPathHistory, 'Show Path History', 'Navigation', () => this.openHistoryFor(this.activePaneId));
    descriptor(COMMAND.showQuickAccess, 'Open Quick Access', 'Navigation', () => this.openQuickAccessFor(this.activePaneId));
    descriptor(
      COMMAND.toggleCurrentPin,
      'Pin or Unpin Current Location',
      'Navigation',
      () => this.togglePanePin(this.activePane),
      () => !!this.activePane?.isDirectoryTab && !!this.activePane?.path,
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
      () => this.activePane?.treeLeft(),
      () => this.activePane?.viewMode === 'tree',
    );
    descriptor(
      COMMAND.treeRight,
      'Tree Expand or Child',
      'Navigation',
      () => this.activePane?.treeRight(),
      () => this.activePane?.viewMode === 'tree',
    );
    descriptor(
      COMMAND.comparePanes,
      'Compare Panes',
      'Pane Control',
      () => this.togglePaneComparison(),
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

    descriptor(COMMAND.toggleCurrentSelection, 'Toggle Current Selection', 'Selection', () => this.activePane?.toggleCurrentSelection(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.extendSelectionUp, 'Extend Selection Up', 'Selection', () => this.activePane?.extendSelection(-1), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.extendSelectionDown, 'Extend Selection Down', 'Selection', () => this.activePane?.extendSelection(1), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.selectByMask, 'Select by Mask', 'Selection', () => this.activePane?.selectByMaskPrompt(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.unselectByMask, 'Unselect by Mask', 'Selection', () => this.activePane?.unselectByMaskPrompt(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.invertSelection, 'Invert Selection', 'Selection', () => this.activePane?.invertSelection(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.selectSameExtension, 'Select Same Extension', 'Selection', () => this.activePane?.selectSameExtension(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.selectFiles, 'Select Visible Files', 'Selection', () => this.activePane?.selectFiles(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.selectFolders, 'Select Visible Folders', 'Selection', () => this.activePane?.selectFolders(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.unselectFiles, 'Unselect Visible Files', 'Selection', () => this.activePane?.unselectFiles(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.unselectFolders, 'Unselect Visible Folders', 'Selection', () => this.activePane?.unselectFolders(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.selectSameType, 'Select Same Type', 'Selection', () => this.activePane?.selectSameType(), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry);
    descriptor(COMMAND.cursorHome, 'Move Cursor to First', 'Navigation', () => this.activePane?.moveCursorHome(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.cursorEnd, 'Move Cursor to Last', 'Navigation', () => this.activePane?.moveCursorEnd(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.cursorPageUp, 'Page Up', 'Navigation', () => this.activePane?.moveCursorPage(-1), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.cursorPageDown, 'Page Down', 'Navigation', () => this.activePane?.moveCursorPage(1), () => !!this.activePane?.isDirectoryTab);

    descriptor(COMMAND.openBasket, 'Open Selection Basket', 'Tools', () => this.selectionBasket?.show());
    descriptor(COMMAND.addToBasket, 'Add to Selection Basket', 'Selection', () => this.addActiveSelectionToBasket(), () => !!this.activePane?.isDirectoryTab && (!!this.activePane?.hasSelection || !!this.activePane?.cursorPath));
    descriptor(COMMAND.clearBasket, 'Clear Selection Basket', 'Tools', () => this.selectionBasketService.clear(), () => this.selectionBasketService.snapshot.length > 0);

    descriptor(COMMAND.openOperations, 'Open Operations Center', 'Tools', () => this.openTools('operations'));
    descriptor(COMMAND.diskUsage, 'Disk Usage', 'Tools', () => this.openTools('disk'), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.findDuplicates, 'Find Duplicate Files', 'Tools', () => this.openTools('duplicates'), () => !!this.activePane?.isDirectoryTab);
    descriptor(
      COMMAND.batchRename,
      'Batch Rename',
      'File Operations',
      () => this.batchRenameActiveSelection(),
      () =>
        !!this.activePane?.isDirectoryTab &&
        this.activePane.sourceCapabilities.rename,
    );
    descriptor(
      COMMAND.extractArchiveSelection,
      'Extract Archive Selection',
      'Archive',
      () => this.extractArchiveSelectionToFolder(),
      () => !!this.activePane?.isInArchive && !!this.activePane?.hasSelection,
    );
    descriptor(
      COMMAND.extractArchiveToOther,
      'Extract Archive Selection to Other Pane',
      'Archive',
      () => this.extractArchiveSelectionToOther(),
      () =>
        !!this.activePane?.isInArchive &&
        !!this.activePane?.hasSelection &&
        !!this.otherPane?.isDirectoryTab &&
        !!this.otherPane?.sourceCapabilities.create,
    );

    descriptor(COMMAND.globalSearch, 'Open Global Search', 'Search & Filter', () => this.openGlobalSearch(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.quickPathJump, 'Quick Path Jump', 'Navigation', () => this.quickPathJump(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.quickView, 'Quick View', 'File Operations', () => this.quickViewActive(), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.openWithExternal, 'Open with External App', 'File Operations', () => this.openActiveExternally(), () => !!this.activePane?.isDirectoryTab && !!this.activePane?.cursorEntry && this.activePane.cursorEntry.type !== FileType.DIRECTORY);
    descriptor(COMMAND.remoteConnect, 'Connect Remote Location', 'Navigation', () => this.connectRemote());
    this.commandPaletteCommands = catalog;
    this.cdr.markForCheck();
  }

  private activeViewer(): any {
    const pane = this.activePane;
    if (!pane || pane.activeTab?.kind !== 'viewer') return null;
    return pane.mountedViewer;
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
    if (this.activePane) this.activePane.lastAction = `viewer: ${method}`;
    this.cdr.markForCheck();
  }

  private canDeleteActiveSelection(): boolean {
    const pane = this.activePane;
    if (!pane) return false;
    if (pane.activeTab?.kind === 'viewer') return this.hasViewerMethod('deleteSelected');
    return pane.canDelete;
  }

  private deleteActiveSelection(): void {
    const pane = this.activePane;
    if (!pane) return;
    if (pane.activeTab?.kind === 'viewer') {
      this.invokeViewer('deleteSelected');
      return;
    }
    void pane.deleteSelected();
  }

  private canTransfer(operation: 'copy' | 'move'): boolean {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination || !source.hasSelection || !destination.path) {
      return false;
    }

    if (source.isInArchive) {
      if (!destination.sourceCapabilities.create) return false;
      if (operation === 'copy') return true;

      // ArchiveService.move can only truly remove the source from a top-level
      // archive. Nested archive moves are intentionally not presented as moves.
      return (source.currentArchiveFrame?.nestedChain.length ?? 1) === 0;
    }

    return canTransferBetween(
      operation,
      source.sourceCapabilities,
      destination.sourceCapabilities,
    ).allowed;
  }

  private async transferToOther(operation: 'copy' | 'move'): Promise<void> {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination) return;

    const selected = Array.from(source.selectedPaths);
    if (selected.length === 0) return;

    if (source.isInArchive) {
      if (!destination.sourceCapabilities.create) {
        await this.confirmDialog.alert({
          title: 'Archive Transfer',
          message: 'The destination pane is read-only.',
        });
        return;
      }

      const result = await source.transferArchiveSelectionTo(
        destination.path,
        operation,
      );
      if (!result) return;

      this.lastTransfer =
        `archive ${operation}: ${result.succeeded}/${result.total}`;
      destination.lastAction =
        `received archive ${operation}: ${result.succeeded}/${result.total}`;

      await destination.refresh();

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

    const decision = canTransferBetween(operation, source.sourceCapabilities, destination.sourceCapabilities);
    if (!decision.allowed) {
      await this.confirmDialog.alert(transferBlockedMessage(operation, decision));
      return;
    }

    try {
      const items = await Promise.all(selected.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.api.joinPaths([destination.path, this.leafName(sourcePath)]),
      })));

      const result = operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      this.lastTransfer = `${operation}: ${result.succeeded}/${result.total}`;
      source.lastAction = `${operation} to ${this.otherPaneId()}: ${result.succeeded}/${result.total}`;
      destination.lastAction = `received ${operation}: ${result.succeeded}/${result.total}`;

      await destination.refresh();
      if (operation === 'move') {
        await source.refresh();
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