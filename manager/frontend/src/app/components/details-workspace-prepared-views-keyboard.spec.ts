import { readFileSync } from 'node:fs';

const host = readFileSync(new URL('./tool-workspace-host.component.ts', import.meta.url), 'utf8');
const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

describe('Details workspace prepared views and keyboard navigation', () => {
  it('owns the prepared-view selector in the Details workspace rather than DirectoryPane', () => {
    expect(host).toContain('detailsWorkspaceViewPresets');
    expect(host).toContain('aria-label="Details view"');
    expect(host).toContain('[detailsColumns]="detailsWorkspaceColumns"');
    expect(directory).not.toContain('detailsViewPresets');
    expect(directory).not.toContain('details-view-select');
  });

  it('provides prepared views instead of individual column toggles', () => {
    for (const id of ['brief', 'standard', 'full', 'security', 'times']) {
      expect(host).toContain(`id: '${id}'`);
    }
    expect(host).not.toContain('type="checkbox"');
  });

  it('uses Tab and Shift+Tab inside the Details workspace to move between tree and details', () => {
    expect(host).toContain("event.key !== 'Tab'");
    expect(host).toContain("this.detailsWorkspacePane === 'tree' ? 'details' : 'tree'");
    expect(host).toContain('data-details-workspace-pane="tree"');
    expect(host).toContain('data-details-workspace-pane="details"');
  });

  it('does not add a duplicate Shift+Tab binding to the ordinary dual-pane manager', () => {
    expect(dual).not.toContain('reassembly.manager.switchPane.shiftTab');
  });

  it('does not render support/primary role notes in workspace headers', () => {
    expect(host).not.toContain("surface.role || 'primary'");
  });
});
