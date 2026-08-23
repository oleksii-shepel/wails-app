import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('BinPane template integrity', () => {
  it('keeps the entry opening tag complete', () => {
    expect(source).toContain('[class.drop-target]="dragOverPath === entry.path"');
    expect(source).toContain('[class.drop-invalid]="dragOverPath === entry.path && dropIntent === \'invalid\'"');
    expect(source).toContain('[class.drop-copy]="dragOverPath === entry.path && dropIntent === \'copy\'"');
    expect(source).toContain('[class.drop-move]="dragOverPath === entry.path && dropIntent === \'move\'"');
    expect(source).toContain('[style.grid-template-columns]="viewMode === \'details\' ? detailsGridTemplate : null">');
  });

  it('renders balanced Bin group/revision leading markup', () => {
    expect(source).toContain('<ng-container *ngIf="isBinLocation; else normalEntryIcon">');
    expect(source).toContain('<ng-template #normalEntryIcon>');
    expect(source).toContain('*ngIf="!isBinGroupHeader(entry)" class="icon"');
  });

  it('uses current group/revision class bindings', () => {
    expect(source).toContain('[class.bin-group-header]="isBinLocation && isBinGroupHeader(entry)"');
    expect(source).toContain('[class.bin-revision-row]="isBinLocation && !isBinGroupHeader(entry)"');
  });
});
