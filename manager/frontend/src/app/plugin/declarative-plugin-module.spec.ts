import type { DeclarativePluginModule } from './declarative-plugin-module';

describe('DeclarativePluginModule input contributions', () => {
  it('accepts keyboard and mouse bindings as data', () => {
    const module: DeclarativePluginModule = {
      schemaVersion: 1,
      commands: [{
        id: 'refresh',
        targetCommandId: 'pane.nav.refresh',
      }],
      bindings: [
        {
          id: 'refresh-key',
          commandId: 'refresh',
          trigger: {
            device: 'keyboard',
            chord: { steps: [{ key: 'r', modifiers: ['ctrl', 'alt'] }] },
          },
        },
        {
          id: 'refresh-mouse',
          commandId: 'refresh',
          trigger: {
            device: 'mouse',
            gesture: { type: 'doubleClick', button: 'middle' },
          },
        },
      ],
    };

    expect(module.bindings?.length).toBe(2);
  });
});