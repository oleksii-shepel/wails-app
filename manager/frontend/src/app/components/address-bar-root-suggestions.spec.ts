import { AddressBarEditComponent } from './address-bar-edit.component';

describe('address bar root suggestions', () => {
  function component(availableRoots: readonly string[]): AddressBarEditComponent {
    const roots = new Set(availableRoots.map(path => path.toLowerCase()));
    const panelSources = {
      resolve: ({ path }: { path: string }) => ({
        source: {
          id: roots.has(path.toLowerCase()) ? `source.${path.slice(0, -3)}` : 'filesystem',
        },
      }),
    };

    const instance = new AddressBarEditComponent(
      {} as any,
      {} as any,
      {} as any,
      {} as any,
      panelSources as any,
      {} as any,
      {} as any,
      {} as any,
      {} as any,
      {} as any,
    );

    (instance as any).syntheticRootSuggestions = [
      { path: 'filesystem://c', displayName: 'Local Disk (C:)' },
      { path: 'filesystem://d', displayName: 'Data (D:)' },
    ];
    return instance;
  }

  function paths(instance: AddressBarEditComponent, query = ''): string[] {
    return (instance as any).buildAliasRootSuggestions(query)
      .map((entry: { path: string }) => entry.path);
  }

  it('shows filesystem roots plus built-in autorun, environment and network roots', () => {
    const instance = component(['autorun://', 'environment://', 'network://']);

    expect(paths(instance)).toEqual([
      'autorun://',
      'environment://',
      'network://',
      'filesystem://c',
      'filesystem://d',
    ]);
  });

  it('shows Registry only while a registered PanelSource owns registry://', () => {
    const installed = component(['autorun://', 'environment://', 'network://', 'registry://']);
    const absent = component(['autorun://', 'environment://', 'network://']);

    expect(paths(installed)).toContain('registry://');
    expect(paths(absent)).not.toContain('registry://');
  });

  it('filters root suggestions by scheme or friendly display name while typing', () => {
    const instance = component(['autorun://', 'environment://', 'network://', 'registry://']);

    expect(paths(instance, 'reg')).toEqual(['registry://']);
    expect(paths(instance, 'env')).toEqual(['environment://']);
    expect(paths(instance, 'net')).toEqual(['network://']);
    expect(paths(instance, 'auto')).toEqual(['autorun://']);
    expect(paths(instance, 'data')).toEqual(['filesystem://d']);
  });
});
