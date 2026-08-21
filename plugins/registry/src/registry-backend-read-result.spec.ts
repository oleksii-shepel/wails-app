import { RegistryBackendClient } from './universal/registry-backend';

describe('Registry backend read normalization', () => {
  it('maps native data payloads to content for the editor flow', async () => {
    const transport = {
      call: jasmine.createSpy('call').and.resolveTo({
        path: 'registry://HKEY_CURRENT_USER/Software/Test',
        type: 'REG_SZ',
        data: 'hello registry',
      }),
    };

    const backend = new RegistryBackendClient(transport);
    const result = await backend.read('registry://HKEY_CURRENT_USER/Software/Test');

    expect(transport.call).toHaveBeenCalledWith('registry.read', {
      path: 'registry://HKEY_CURRENT_USER/Software/Test',
    });
    expect(result.content).toBe('hello registry');
  });
});
