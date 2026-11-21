import '@angular/compiler';
import type { WorkspaceConfig } from '../shared/protocol-enhanced';
import { WorkspaceService, type RecentlyClosedTab } from './workspace.service';

describe('Phase 78 workspace closed-tab contract', () => {
  it('exposes recent closed-tab recovery APIs', () => {
    expect(WorkspaceService.prototype.getRecentlyClosedTabs).toBeDefined();
    expect(WorkspaceService.prototype.reopenLastClosedTab).toBeDefined();
    expect(WorkspaceService.prototype.reopenClosedTabById).toBeDefined();
    expect(WorkspaceService.prototype.clearRecentlyClosedTabs).toBeDefined();
  });
});

function setup(workspaceId = 'active') {
  const folder = (id: string) => ({ id, path: '/same', title: 'Folder', isActive: id === 'anchor' });
  const workspace = {
    id: workspaceId,
    leftPane: { tabs: [folder('anchor'), folder('first'), folder('second')], activeTabId: 'anchor' },
    rightPane: { tabs: [] },
  } as unknown as WorkspaceConfig;
  const recovery: RecentlyClosedTab[] = [];
  const updateWorkspace = jasmine.createSpy('updateWorkspace').and.callFake(
    (_id: string, update: Partial<WorkspaceConfig>) => {
      Object.assign(workspace, update);
      return true;
    },
  );
  const service = Object.create(WorkspaceService.prototype) as WorkspaceService;
  Object.assign(service, {
    activeWorkspaceSubject: { value: workspace },
    recentlyClosedTabs: recovery,
    closedTabsRevision: () => 0,
    updateWorkspace,
    clearArchiveState() {},
    persistRecentlyClosedTabs() {},
    generateId: () => 'restored',
  });
  return { service, workspace, updateWorkspace };
}

describe('WorkspaceService closed-tab recovery', () => {
  it('keeps distinct tabs closed at the same location', () => {
    const { service } = setup();
    service.closeTab('left', 'first');
    service.closeTab('left', 'second');

    expect(service.getRecentlyClosedTabs().map(entry => entry.tab.id)).toEqual(['second', 'first']);
  });

  it('restores a tab at its saved position and activates it', () => {
    const { service, workspace } = setup();
    service.closeTab('left', 'first');

    expect(service.reopenLastClosedTab('left')?.tab.id).toBe('restored');
    expect(workspace.leftPane.tabs.map(tab => tab.id)).toEqual(['anchor', 'restored', 'second']);
    expect(workspace.leftPane.activeTabId).toBe('restored');
    expect(service.getRecentlyClosedTabs()).toEqual([]);
  });

  it('keeps the closed tab recoverable when the workspace update fails', () => {
    const { service, workspace, updateWorkspace } = setup();
    service.closeTab('left', 'first');
    updateWorkspace.and.returnValue(false);

    expect(service.reopenLastClosedTab('left')).toBeNull();
    expect(workspace.leftPane.tabs.map(tab => tab.id)).toEqual(['anchor', 'second']);
    expect(workspace.leftPane.tabs[0].isActive).toBeTrue();
    expect(service.getRecentlyClosedTabs()[0].tab.id).toBe('first');
  });
});
