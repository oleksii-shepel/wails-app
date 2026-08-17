import { WorkspaceService } from './workspace.service';

describe('Phase 78 workspace closed-tab contract', () => {
  it('exposes recent closed-tab recovery APIs', () => {
    expect(WorkspaceService.prototype.getRecentlyClosedTabs).toBeDefined();
    expect(WorkspaceService.prototype.reopenLastClosedTab).toBeDefined();
    expect(WorkspaceService.prototype.reopenClosedTabById).toBeDefined();
    expect(WorkspaceService.prototype.clearRecentlyClosedTabs).toBeDefined();
  });
});
