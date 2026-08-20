  async setTrusted(pluginId: string, trusted: boolean): Promise<PluginPackageState> {
    let state = await this.installation.setTrusted(pluginId, trusted);

    // Explicit trust approves this exact plugin version and its requested
    // permissions, so make the plugin usable immediately.
    if (trusted && !state.enabled) {
      state = await this.installation.setEnabled(pluginId, true);
    }

    await this.sync();
    return state;
  }
