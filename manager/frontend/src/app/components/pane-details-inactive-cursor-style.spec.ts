import { readFileSync } from 'node:fs';

describe('PaneDetails inactive pane cursor styling', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('does not render a cursor decoration when the pane is inactive', () => {
    expect(source).not.toContain(':host-context(.pane:not(.active-pane)) .details-row.cursor');
  });

  it('keeps cursor decoration scoped to the active pane', () => {
    expect(source).toContain(':host-context(.pane.active-pane) .details-row.cursor');
  });

  it('keeps inactive selection separate from cursor', () => {
    expect(source).toContain(':host-context(.pane:not(.active-pane)) .details-row.selected');
  });
});
