import { SimpleChange } from '@angular/core';
import { fn } from '../testing/jasmine-test-helpers';
import { InputCenterComponent } from './input-center.component';
import type { InputBindingConflict, InputBindingDefinition } from '../services/input-system.types';

function binding(id: string, key: string): InputBindingDefinition {
  return {
    id,
    commandId: `command.${id}`,
    contextId: 'global',
    source: 'default',
    description: id,
    category: 'Test',
    enabled: true,
    trigger: { device: 'keyboard', chord: { steps: [{ key }] } },
  };
}

describe('InputCenterComponent presentation cache', () => {
  it('builds bindings and conflicts once when opened and reuses the conflict index per row', () => {
    const bindings = [binding('one', 'a'), binding('two', 'a')];
    const conflicts: InputBindingConflict[] = [{
      bindingId: 'one',
      conflictingBindingId: 'two',
      winnerBindingId: 'one',
      loserBindingId: 'two',
      reason: 'same-context',
    }];
    const customization = {
      bindings: fn(() => bindings),
      conflicts: fn(() => conflicts),
      hasOverride: fn(() => false),
    } as any;
    const subscription = { unsubscribe: fn() };
    const engine = {
      bindings$: { subscribe: fn(() => subscription) },
      formatInputTrigger: fn(() => 'A'),
      getBinding: fn(),
    } as any;
    const component = new InputCenterComponent(
      customization,
      engine,
      {} as any,
      { markForCheck: fn() } as any,
    );

    component.visible = true;
    component.ngOnChanges({ visible: new SimpleChange(false, true, false) });

    expect(customization.bindings).toHaveBeenCalledTimes(1);
    expect(customization.conflicts).toHaveBeenCalledTimes(1);
    expect(component.visibleBindings.length).toBe(2);

    for (let i = 0; i < 20; i += 1) {
      expect(component.conflictsFor('one').length).toBe(1);
    }

    expect(customization.conflicts).toHaveBeenCalledTimes(1);
    component.ngOnDestroy();
    expect(subscription.unsubscribe).toHaveBeenCalled();
  });
});
