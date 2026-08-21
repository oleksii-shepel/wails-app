import { readFileSync } from 'node:fs';

const details = readFileSync(
  new URL('./file-details.component.ts', import.meta.url),
  'utf8',
);

describe('Directory shared pane-table presentation', () => {
  it('consumes core pane table primitives', () => {
    expect(details).toContain("from '@reassembly/core/ui/panes'");
    expect(details).toContain('<reassembly-pane-table');
    expect(details).toContain('reassemblyPaneTableHeaderRow');
    expect(details).toContain('reassemblyPaneTableRow');
    expect(details).toContain('reassemblyPaneTableCell');
  });
});
