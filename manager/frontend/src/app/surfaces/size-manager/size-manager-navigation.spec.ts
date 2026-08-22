import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./size-manager-surface.component.ts', import.meta.url), 'utf8');

describe('Size Manager navigation', () => {
  it('keeps the launch root separate from the current path', () => {
    expect(source).toContain("private analysisRootPath = '';");
    expect(source).toContain('const root = this.analysisRootPath;');
    expect(source).toContain('this.isWithinAnalysisRoot(path)');
  });

  it('provides back, forward, up and breadcrumb navigation', () => {
    expect(source).toContain('goBack(): void');
    expect(source).toContain('goForward(): void');
    expect(source).toContain('goUp(): void');
    expect(source).toContain('navigateBreadcrumb(path: string): void');
    expect(source).toContain('navigationBreadcrumbs');
  });

  it('keeps selection separate from directory activation', () => {
    expect(source).toContain('(dblclick)="onEntryDblClick(e)"');
    expect(source).toContain('(keydown.enter)="onEntryKeydown($event, e)"');
    expect(source).toContain('onEntryClick(e: DiskUsageEntry): void {\n    this.selectEntry(e);\n  }');
  });
});
