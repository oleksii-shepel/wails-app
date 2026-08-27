import { readFileSync } from 'node:fs';

describe('Recycle Bin grouped virtual scrolling', () => {
  const pane = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');
  const grouped = readFileSync(new URL('./bin-grouped-view.component.ts', import.meta.url), 'utf8');

  it('renders only the current virtual slice in grouped mode', () => {
    expect(pane).toContain('[entries]="renderedEntries"');
    expect(pane).toContain('[beforePx]="listVirtualWindow.enabled ? listVirtualWindow.beforePx : 0"');
    expect(pane).toContain('[afterPx]="listVirtualWindow.enabled ? listVirtualWindow.afterPx : 0"');
    expect(grouped).toContain('*ngFor="let entry of entries; trackBy: trackEntry"');
  });

  it('uses the grouped viewport as the virtual-scroll owner', () => {
    expect(grouped).toContain('#viewport');
    expect(grouped).toContain('get viewportElement(): HTMLElement | null');
    expect(pane).toContain('this.binGroupedView?.viewportElement');
    expect(pane).toContain('(scroll)="onContentScroll($event)"');
  });

  it('uses grouped row geometry for window calculations', () => {
    expect(pane).toContain('private readonly binGroupedRowExtent = 26;');
    expect(pane).toContain('groupedBin ? this.binGroupedRowExtent : this.listRowExtent');
    expect(pane).toContain('groupedBin ? 32 : 48');
  });

  it('restores grouped scroll position through the grouped viewport', () => {
    expect(grouped).toContain('setScrollTop(value: number): void');
    expect(pane).toContain('this.binGroupedView?.setScrollTop(scrollTop)');
  });
});
