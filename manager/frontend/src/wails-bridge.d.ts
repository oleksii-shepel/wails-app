type WailsGeneratedAppBindings = typeof import('../wailsjs/go/app/App');
type WailsAppBindings = WailsGeneratedAppBindings & {
  BuildSearchAlias(path: string, pattern: string, recursive: boolean): Promise<string>;
};
type WailsRuntimeBindings = Pick<
  typeof import('../wailsjs/runtime/runtime'),
  'EventsOn' | 'EventsOff' | 'EventsEmit'
>;

declare global {
  interface Window {
    go: {
      app: {
        App: WailsAppBindings;
      };
    };
    runtime: WailsRuntimeBindings;
  }
}

export {};