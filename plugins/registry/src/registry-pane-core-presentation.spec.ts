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

  it('uses directory-like compact row and header metrics', () => {
    expect(source).toContain('min-height: 24px;');
    expect(source).toContain('height: 24px;');
    expect(source).toContain('min-height: 28px;');
    expect(source).toContain('height: 28px;');
    expect(source).not.toContain('min-height: 52px;');
  });

  it('keeps ordinary registry rows single-line and table-like', () => {
    expect(source).toContain('class="registry-cell name"');
    expect(source).toContain('class="registry-cell kind"');
    expect(source).toContain('class="registry-cell type"');
    expect(source).toContain('class="registry-cell data"');
    expect(source).not.toContain('class="badge"');
    expect(source).not.toContain('class="meta"');
    expect(source).not.toContain('class="name-line"');
  });

  it('does not reintroduce dashboard chrome inside the collection pane', () => {
    expect(source).not.toContain('registry-surface-header');
    expect(source).not.toContain('registry-section-label');
    expect(source).not.toContain('registry-value-inspector');
    expect(source).not.toContain('border-radius: 999px');
  });

  it('shows the details header at the registry root too', () => {
    expect(source).toContain('*ngIf="viewMode === 'details'"');
    expect(source).not.toContain('viewMode === 'details' && !isRootView');
  });
});
