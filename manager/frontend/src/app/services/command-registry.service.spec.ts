import type { NgZone } from '@angular/core';
import { CommandRegistryService } from './command-registry.service';

function zone(): NgZone {
  return { run: <T>(fn: () => T) => fn() } as unknown as NgZone;
}

describe('CommandRegistryService', () => {
  it('restores the previous owner when the active override is removed', () => {
    const registry = new CommandRegistryService(zone());
    const calls: string[] = [];

    registry.registerDescriptor({ id: 'file.open', title: 'Open', execute: () => calls.push('core') }, 'core');
    registry.registerDescriptor({ id: 'file.open', title: 'Open custom', execute: () => calls.push('plugin') }, 'plugin');

    expect(registry.dispatch('file.open')).toBeTrue();
    expect(calls).toEqual(['plugin']);
    expect(registry.getDefinition('file.open')?.title).toBe('Open custom');

    registry.unregister('file.open', 'plugin');
    expect(registry.dispatch('file.open')).toBeTrue();
    expect(calls).toEqual(['plugin', 'core']);
    expect(registry.getDefinition('file.open')?.title).toBe('Open');
  });

  it('evaluates enablement against the actual invocation context', () => {
    const registry = new CommandRegistryService(zone());
    registry.registerDescriptor({
      id: 'selection.delete',
      title: 'Delete',
      execute: () => undefined,
      isEnabled: invocation => Array.isArray(invocation?.context?.['paths']) && invocation!.context!['paths'].length > 0,
    });

    expect(registry.getState('selection.delete')?.enabled).toBeFalse();
    expect(registry.getState('selection.delete', {
      source: 'context-menu',
      context: { paths: ['C:/one.txt'] },
    })?.enabled).toBeTrue();
  });

  it('resolves names and rejects ambiguous aliases', () => {
    const registry = new CommandRegistryService(zone());
    registry.registerDescriptor({ id: 'one', title: 'One', name: 'First', aliases: ['go'], execute: () => undefined });
    expect(registry.resolve('first')).toBe('one');
    expect(registry.resolve('GO')).toBe('one');

    registry.registerDescriptor({ id: 'two', title: 'Two', aliases: ['go'], execute: () => undefined });
    expect(registry.resolve('go')).toBeUndefined();
  });
});