import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Prepared Details views', () => {
  it('offers prepared layouts instead of per-column controls', () => {
    expect(source).toContain('class="details-view-select"');
    expect(source).toContain('aria-label="Details layout"');
    for (const label of ['Brief', 'Standard', 'Full', 'Security', 'Times']) {
      expect(source).toContain(`label: '${label}'`);
    }
    expect(source).not.toContain('aria-label="Details columns"');
  });

  it('persists the selected prepared view independently per pane', () => {
    expect(source).toContain('`reassembly.details-view.${this.paneId}`');
    expect(source).toContain('localStorage.setItem(this.detailsViewStorageKey');
  });

  it('renders preset columns through the shared Details host', () => {
    expect(source).toContain('[detailsColumns]="visiblePaneDetailsColumns"');
    expect(source).toContain('DETAILS_VIEW_PRESET_FIELDS');
  });
});
