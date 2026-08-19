# Registry Plugin

Angular library for the `builtin.registry` frontend plugin.

## Build

From the repository root:

```bash
npm install
npm run build:plugins
```

The library is compiled with `ng-packagr` into `plugins/registry/dist` using Angular
Package Format. Manager consumes that compiled package; it must not import plugin
source TypeScript directly.

For plugin development:

```bash
npm run build:watch --workspace @reassembly/registry-plugin
```

The Registry VFS/elevation/backend remains in `manager` until a backend plugin ABI
is introduced.
