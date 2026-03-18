type WailsAppBindings = typeof import('../wailsjs/go/app/App');
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