export function rememberFocusedElement(): HTMLElement | null {
  const active = document.activeElement;
  return active instanceof HTMLElement ? active : null;
}

export function restoreFocusedElement(element: HTMLElement | null): void {
  queueMicrotask(() => {
    if (element?.isConnected) {
      element.focus({ preventScroll: true });
    }
  });
}
