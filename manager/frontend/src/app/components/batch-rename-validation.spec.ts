import '@angular/compiler';
import type { BatchRenameRequest } from '../services/batch-rename-dialog.service';
import { BatchRenameDialogComponent } from './batch-rename-dialog.component';

function preview(paths: string[], pattern: string) {
  const component = Object.create(BatchRenameDialogComponent.prototype) as BatchRenameDialogComponent;
  const request = {
    items: paths.map(path => ({ entry: { path, name: path.split('/').at(-1)! } })),
  } as unknown as BatchRenameRequest;
  Object.assign(component, {
    service: { activeRequest$: { value: request } },
    startAt: 1,
    increment: 1,
    findText: '',
  });
  component.applyPreset(pattern);
  return component;
}

describe('Batch Rename validation', () => {
  it('marks all rows that would share a Windows destination name', () => {
    const component = preview(['filesystem://c/docs/Report.txt', 'filesystem://c/docs/report.txt'], '{name}{.ext}');
    expect(component.conflictCount).toBe(2);
  });

  it('allows identical names destined for different folders', () => {
    const component = preview(['/one/a.txt', '/two/b.txt'], 'report.txt');
    expect(component.conflictCount).toBe(0);
  });
});
