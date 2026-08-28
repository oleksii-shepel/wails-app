import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

describe('Pane Details legacy header columns', () => {
  it('uses the old details header viewport and span column structure', () => {
    expect(source).toContain('class="details-header-viewport" role="rowgroup"');
    expect(source).toContain('class="details-header" role="row" [style.grid-template-columns]="columnGridTemplate"');
    expect(source).toContain('class="header-column"');
    expect(source).not.toContain('class="details-cell header-cell"');
  });

  it('renders the shared resize delimiter directly inside every header column', () => {
    expect(source).toContain('<i\n              title="Resize column"');
    expect(source).toContain('[paneDetailsResizeHandle]="col.key"');
    expect(source).toContain('(paneDetailsWidthCommit)="onColumnWidthCommit($event)"');
    expect(source).not.toContain('class="view-divider"');
  });

  it('keeps old header surface geometry and visible column gutters', () => {
    expect(source).toContain('.details-header-viewport {');
    expect(source).toContain('width: max-content;');
    expect(source).toContain('min-width: 100%;');
    expect(source).toContain('padding: 0 8px;');
    expect(source).toContain('column-gap: var(--pane-column-gutter, 12px);');
  });
});


describe('legacy Details header visual contract', () => {
  it('keeps legacy normal-weight header typography without table-header hover fill', () => {
    expect(source).toContain('font-weight: 400;');
    expect(source).not.toContain('.header-column.sortable:hover');
  });

  it('reserves the legacy 28px icon gutter before the Name label', () => {
    expect(source).toContain('.header-column:first-child');
    expect(source).toContain('padding-left: 30px;');
  });
  it('paints legacy compact resize delimiters instead of full-height table borders', () => {
    expect(source).toContain('.header-column > .pane-details-resize-handle::before');
    expect(source).toContain('top: 5px;');
    expect(source).toContain('bottom: 5px;');
    expect(source).toContain('background: transparent !important;');
  });

  it('pulls the sort indicator slightly toward its label', () => {
    expect(source).toContain('margin-left: calc(var(--vsc-padding-md) - 3px);');
  });


});
