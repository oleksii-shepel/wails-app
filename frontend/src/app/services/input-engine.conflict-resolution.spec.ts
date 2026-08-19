import { NgZone } from '@angular/core';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';

function createEngine(): InputEngineService {
  const zone = new NgZone({ enableLongStackTrace: false });
  return new InputEngineService(zone, new CommandRegistryService(zone));
}

function bind(
  engine: InputEngineService,
  id: string,
  key: string,
  when?: string,
): void {
  const commandId = `command.${id}`;
  engine.registerCommandDescriptor({ id: commandId, title: id, execute: () => undefined });
  engine.registerBinding({
    id,
    commandId,
    contextId: 'global',
    source: 'default',
    description: id,
    when,
    trigger: { device: 'keyboard', chord: { steps: [{ key }] } },
  });
}

describe('InputEngineService shortcut conflict resolution', () => {
  it('does not report mutually exclusive when clauses as conflicts', () => {
    const engine = createEngine();
    bind(engine, 'test.viewer.left', 'f13', 'viewerSurface');
    bind(engine, 'test.tree.left', 'f13', '!typing && !viewerSurface');

    const conflicts = engine.inspectBindingConflicts().filter(conflict =>
      conflict.bindingId.startsWith('test.') || conflict.conflictingBindingId.startsWith('test.')
    );

    expect(conflicts).toEqual([]);
  });

  it('uses stable binding identity instead of registration order for exact ties', () => {
    const first = createEngine();
    bind(first, 'test.zeta', 'f14');
    bind(first, 'test.alpha', 'f14');

    const second = createEngine();
    bind(second, 'test.alpha', 'f14');
    bind(second, 'test.zeta', 'f14');

    expect(first.resolveKeyStroke({ key: 'f14' })?.binding.id).toBe('test.alpha');
    expect(second.resolveKeyStroke({ key: 'f14' })?.binding.id).toBe('test.alpha');
  });

  it('does not report collisions across mutually exclusive surfaces', () => {
    const engine = createEngine();
    engine.registerContext({
      id: 'overlay.one', kind: 'overlay', priority: 100, parentId: 'global', exclusive: true, description: 'One',
    });
    engine.registerContext({
      id: 'overlay.two', kind: 'overlay', priority: 100, parentId: 'global', exclusive: true, description: 'Two',
    });

    const commandOne = 'command.overlay.one';
    const commandTwo = 'command.overlay.two';
    engine.registerCommandDescriptor({ id: commandOne, title: commandOne, execute: () => undefined });
    engine.registerCommandDescriptor({ id: commandTwo, title: commandTwo, execute: () => undefined });
    engine.registerBinding({
      id: 'test.overlay.one.escape', commandId: commandOne, contextId: 'overlay.one', source: 'default',
      description: 'Close one', trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
    });
    engine.registerBinding({
      id: 'test.overlay.two.escape', commandId: commandTwo, contextId: 'overlay.two', source: 'default',
      description: 'Close two', trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
    });

    const conflicts = engine.inspectBindingConflicts().filter(conflict =>
      conflict.bindingId.startsWith('test.overlay') || conflict.conflictingBindingId.startsWith('test.overlay')
    );
    expect(conflicts).toEqual([]);
  });

  it('does not report cross-context shortcut reuse as a conflict', () => {
    const engine = createEngine();

    engine.registerContext({
      id: 'dialog.parent',
      kind: 'dialog',
      priority: 900,
      parentId: 'global',
      description: 'parent dialog',
    });
    engine.registerContext({
      id: 'dialog.child',
      kind: 'dialog',
      priority: 910,
      parentId: 'dialog.parent',
      description: 'child dialog',
    });

    engine.registerBinding({
      id: 'test.parent.enter',
      commandId: 'test.parent.open',
      contextId: 'dialog.parent',
      source: 'default',
      description: 'Parent action',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } },
    });
    engine.registerBinding({
      id: 'test.child.enter',
      commandId: 'test.child.apply',
      contextId: 'dialog.child',
      source: 'default',
      description: 'Child action',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } },
    });

    const conflicts = engine.inspectBindingConflicts().filter(conflict =>
      conflict.bindingId.startsWith('test.') ||
      conflict.conflictingBindingId.startsWith('test.')
    );

    expect(conflicts).toEqual([]);
  });

  it('does not report same-command duplicate registrations as conflicts', () => {
    const engine = createEngine();

    engine.registerBinding({
      id: 'test.alias.one',
      commandId: 'test.same.command',
      contextId: 'global',
      source: 'default',
      description: 'Alias one',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f19' }] } },
    });
    engine.registerBinding({
      id: 'test.alias.two',
      commandId: 'test.same.command',
      contextId: 'global',
      source: 'default',
      description: 'Alias two',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f19' }] } },
    });

    const conflicts = engine.inspectBindingConflicts().filter(conflict =>
      conflict.bindingId.startsWith('test.alias.') ||
      conflict.conflictingBindingId.startsWith('test.alias.')
    );

    expect(conflicts).toEqual([]);
  });

  it('still reports a same-context equal-tier collision between different commands', () => {
    const engine = createEngine();

    engine.registerBinding({
      id: 'test.real.one',
      commandId: 'test.real.commandOne',
      contextId: 'global',
      source: 'default',
      description: 'Real one',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f20' }] } },
    });
    engine.registerBinding({
      id: 'test.real.two',
      commandId: 'test.real.commandTwo',
      contextId: 'global',
      source: 'default',
      description: 'Real two',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f20' }] } },
    });

    const conflicts = engine.inspectBindingConflicts().filter(conflict =>
      conflict.bindingId.startsWith('test.real.') ||
      conflict.conflictingBindingId.startsWith('test.real.')
    );

    expect(conflicts.length).toBe(1);
    expect(conflicts[0].reason).toBe('same-context');
  });

});