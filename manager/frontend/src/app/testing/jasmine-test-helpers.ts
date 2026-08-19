type AnyFunction = (...args: any[]) => any;

export type MockSpy<T extends AnyFunction = AnyFunction> = jasmine.Spy & T & {
  mockImplementation(impl: T): MockSpy<T>;
  mockResolvedValue(value: Awaited<ReturnType<T>>): MockSpy<T>;
  mockRejectedValue(error: unknown): MockSpy<T>;
  mockRestore(): void;
};

const globalRestorers: Array<() => void> = [];

function enhanceSpy<T extends AnyFunction>(spy: jasmine.Spy): MockSpy<T> {
  const enhanced = spy as MockSpy<T>;

  enhanced.mockImplementation = (impl: T) => {
    spy.and.callFake(impl);
    return enhanced;
  };

  enhanced.mockResolvedValue = (value: Awaited<ReturnType<T>>) => {
    spy.and.returnValue(Promise.resolve(value));
    return enhanced;
  };

  enhanced.mockRejectedValue = (error: unknown) => {
    spy.and.returnValue(Promise.reject(error));
    return enhanced;
  };

  enhanced.mockRestore = () => {
    spy.and.callThrough();
  };

  return enhanced;
}

export function fn<T extends AnyFunction = AnyFunction>(impl?: T): MockSpy<T> {
  const spy = jasmine.createSpy('fn');
  const enhanced = enhanceSpy<T>(spy);
  if (impl) {
    enhanced.mockImplementation(impl);
  }
  return enhanced;
}

export function spyOnFn<T extends Record<string, any>, K extends keyof T & string>(
  target: T,
  method: K,
): MockSpy<Extract<T[K], AnyFunction>> {
  return enhanceSpy<Extract<T[K], AnyFunction>>(spyOn(target, method));
}

export function stubGlobal(name: string, value: unknown): void {
  const hadOwn = Object.prototype.hasOwnProperty.call(globalThis, name);
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, name);
  const previousValue = (globalThis as Record<string, unknown>)[name];

  globalRestorers.push(() => {
    if (descriptor) {
      Object.defineProperty(globalThis, name, descriptor);
      return;
    }

    if (hadOwn) {
      Object.defineProperty(globalThis, name, {
        configurable: true,
        writable: true,
        value: previousValue,
      });
      return;
    }

    delete (globalThis as Record<string, unknown>)[name];
  });

  Object.defineProperty(globalThis, name, {
    configurable: true,
    writable: true,
    value,
  });
}

export function restoreGlobals(): void {
  while (globalRestorers.length > 0) {
    globalRestorers.pop()?.();
  }
}