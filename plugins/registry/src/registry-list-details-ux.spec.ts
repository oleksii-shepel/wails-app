import { readFileSync } from 'node:fs';

describe('Registry list/details UX', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('shows Registry payload directly below its name in List view', () => {
    expect(pane).toContain('class="registry-payload"');
    expect(pane).toContain('registryPayload(entry) as payload');
    expect(pane).toContain("String(entry.registryKind ?? '').toLowerCase() !== 'value'");
    expect(pane).not.toContain('inline-provider-label');
  });

  it('supports details column resizing with persisted widths', () => {
    expect(pane).toContain('onDetailsColumnResizeStart($event');
    expect(pane).toContain("document.body.style.cursor = 'col-resize'");
    expect(pane).toContain('reassembly.detailsColumns:${this.paneId}:${surface}');
    expect(pane).toContain('saveDetailsColumnWidths()');
  });

  it('keeps a small visual gap between Details columns', () => {
    expect(pane).toContain('column-gap: 6px');
    expect(pane).toContain('class="details-resizer"');
  });
});