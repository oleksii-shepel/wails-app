  it('automatically enables a disabled plugin when it is explicitly trusted', async () => {
    const trustedState = state({
      enabled: false,
      trusted: true,
      trustedVersion: '1.0.0',
    });
    const enabledState = state({
      enabled: true,
      trusted: true,
      trustedVersion: '1.0.0',
    });
    const pkg = managed({
      state: enabledState,
      readyToActivate: true,
    });
    const installation = {
      setTrusted: vi.fn(async () => trustedState),
      setEnabled: vi.fn(async () => enabledState),
      refresh: vi.fn(async () => [pkg]),
    } as any;
    const platform = createPlatform(false);
    const coordinator = new PluginLifecycleCoordinatorService(
      installation,
      platform as any,
    );

    const result = await coordinator.setTrusted('example', true);

    expect(installation.setTrusted).toHaveBeenCalledWith('example', true);
    expect(installation.setEnabled).toHaveBeenCalledWith('example', true);
    expect(result.enabled).toBe(true);
  });

  it('does not force enablement when trust is revoked', async () => {
    const untrustedState = state({ enabled: false, trusted: false });
    const pkg = managed({
      state: untrustedState,
      effectiveGrantedPermissions: [],
      missingPermissions: ['commands'],
      readyToActivate: false,
    });
    const installation = {
      setTrusted: vi.fn(async () => untrustedState),
      setEnabled: vi.fn(),
      refresh: vi.fn(async () => [pkg]),
    } as any;
    const platform = createPlatform(false);
    const coordinator = new PluginLifecycleCoordinatorService(
      installation,
      platform as any,
    );

    await coordinator.setTrusted('example', false);

    expect(installation.setEnabled).not.toHaveBeenCalled();
  });
