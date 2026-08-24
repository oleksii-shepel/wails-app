import { readFileSync } from 'node:fs';

describe('SourceChangeObserverService architecture', () => {
  const observer = readFileSync(
    new URL('./source-change-observer.service.ts', import.meta.url),
    'utf8',
  );
  const directory = readFileSync(
    new URL('../components/directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const bin = readFileSync(
    new URL('../components/bin-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('feeds both filesystem and Bin producers into ResourceMutationService', () => {
    expect(observer).toContain("'fs:directoryChanged'");
    expect(observer).toContain("origin: 'filesystem-watch'");
    expect(observer).toContain("origin: 'bin-os-observer'");
    expect(observer).toContain('this.mutations.publish({');
  });

  it('keeps Bin polling shared, adaptive, and inactive without consumers', () => {
    expect(observer).toContain('binObserverCount');
    expect(observer).toContain('BIN_VISIBLE_POLL_MS');
    expect(observer).toContain('BIN_HIDDEN_POLL_MS');
    expect(observer).toContain("document.visibilityState === 'hidden'");
    expect(observer).toContain('if (this.binObserverCount !== 0) return;');
  });

  it('owns the refresh in-flight gate instead of duplicating it in panes', () => {
    expect(observer).toContain('record.refreshing');
    expect(observer).toContain('record.pending');
    expect(directory).not.toContain('mutationRefreshRunning');
    expect(bin).not.toContain('binMutationRefreshRunning');
  });

  it('makes panes register locations without listening to producer events directly', () => {
    expect(directory).toContain('this.sourceChanges.observe(');
    expect(directory).toContain('this.sourceObservation?.setLocation(nextPath);');
    expect(bin).toContain('this.sourceChanges.observe(');
    expect(bin).toContain('this.sourceObservation.setLocation(BIN_ROOT_PATH);');
    expect(directory).not.toContain("EventsOn('fs:directoryChanged'");
    expect(bin).not.toContain("EventsOn('fs:directoryChanged'");
  });
});
