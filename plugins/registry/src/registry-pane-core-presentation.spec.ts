import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Registry shared pane-table presentation', () => {
  it('uses the same core collection primitives as Directory details', () => {
    expect(source).toContain("from '@reassembly/core/ui/panes'");
    expect(source).toContain('<reassembly-pane-table');
    expect(source).toContain('reassemblyPaneTableHeaderRow');
    expect(source).toContain('reassemblyPaneTableRow');
    expect(source).toContain('reassemblyPaneTableCell');
  });

  it('uses directory-like row metrics instead of card rows', () => {
    expect(source).toContain('min-height: 24px;');
    expect(source).toContain('height: 24px;');
    expect(source).not.toContain('min-height: 52px;');
  });
});
