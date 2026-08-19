import { NgZone } from '@angular/core';
import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';

function createEngine(): InputEngineService {
  const zone = new NgZone({ enableLongStackTrace: false });
  return new InputEngineService(zone, new CommandRegistryService(zone));
}

describe('InputEngineService human command names', () => {
  it('resolves canonical names and aliases without changing stable command ids', () => {
    const engine = createEngine();
    engine.registerCommandDescriptor({
      id: 'pane.files.newFolder',
      title: 'New Folder',
      name: 'mkdir',
      aliases: ['new-folder'],
      execute: fn(),
    });

    expect(engine.resolveCommandId('pane.files.newFolder')).toBe('pane.files.newFolder');
    expect(engine.resolveCommandId('mkdir')).toBe('pane.files.newFolder');
    expect(engine.resolveCommandId('NEW-FOLDER')).toBe('pane.files.newFolder');
  });

  it('refuses ambiguous aliases instead of guessing', () => {
    const engine = createEngine();
    engine.registerCommandDescriptor({ id: 'one', title: 'One', name: 'same', execute: fn() });
    engine.registerCommandDescriptor({ id: 'two', title: 'Two', aliases: ['same'], execute: fn() });

    expect(engine.resolveCommandId('same')).toBeUndefined();
  });

  it('dispatches by human-facing name with structured invocation', () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({ id: 'test.open', title: 'Open', name: 'open', execute });

    expect(engine.dispatchCommandByName('open', {
      source: 'console',
      args: { location: 's3://bucket/report.csv' },
    })).toBe(true);
    expect(execute).toHaveBeenCalledWith({
      source: 'console',
      args: { location: 's3://bucket/report.csv' },
    });
  });
});