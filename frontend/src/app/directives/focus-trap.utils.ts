const FOCUSABLE_SELECTOR = [
  'a[href]',
  'area[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  'iframe',
  'object',
  'embed',
  '[contenteditable="true"]',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

function isHiddenByAncestor(element: HTMLElement): boolean {
  return element.closest('[hidden], [inert], [aria-hidden="true"]') !== null;
}

function isRadioTabStop(element: HTMLElement): boolean {
  if (!(element instanceof HTMLInputElement) || element.type !== 'radio' || !element.name || element.checked) {
    return true;
  }

  const ownerDocument = element.ownerDocument;
  const escapedName = typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
    ? CSS.escape(element.name)
    : element.name.replace(/([\\"'\[\]#.:])/g, '\\$1');
  const group = Array.from(ownerDocument.querySelectorAll<HTMLInputElement>(`input[type="radio"][name="${escapedName}"]`))
    .filter(radio => radio.form === element.form);

  return !group.some(radio => radio.checked);
}

export function isFocusableElement(element: HTMLElement): boolean {
  if (!element.isConnected) {
    return false;
  }

  if (element.hasAttribute('disabled') || element.getAttribute('aria-disabled') === 'true') {
    return false;
  }

  if (isHiddenByAncestor(element)) {
    return false;
  }

  if (element.tabIndex < 0) {
    return false;
  }

  const style = getComputedStyle(element);
  if (style.display === 'none' || style.visibility === 'hidden' || style.visibility === 'collapse') {
    return false;
  }

  if (!isRadioTabStop(element)) {
    return false;
  }

  return element.getClientRects().length > 0;
}

export function getFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
    .filter(isFocusableElement);
}

export function focusInitialElement(container: HTMLElement, preferredSelector?: string | null): void {
  const selectorCandidates = [preferredSelector, '[data-dialog-initial-focus]', '[autofocus]']
    .filter((selector): selector is string => typeof selector === 'string' && selector.trim().length > 0);

  for (const selector of selectorCandidates) {
    const candidate = container.querySelector<HTMLElement>(selector);
    if (candidate && isFocusableElement(candidate)) {
      candidate.focus();
      return;
    }
  }

  const firstFocusable = getFocusableElements(container)[0];
  if (firstFocusable) {
    firstFocusable.focus();
    return;
  }

  container.focus();
}

export function moveFocusWithin(container: HTMLElement, delta: number, selector?: string): boolean {
  const focusable = selector
    ? Array.from(container.querySelectorAll<HTMLElement>(selector)).filter(isFocusableElement)
    : getFocusableElements(container);
  if (focusable.length === 0) {
    container.focus();
    return false;
  }

  const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const currentIndex = activeElement && container.contains(activeElement)
    ? focusable.indexOf(activeElement)
    : -1;

  const nextIndex = currentIndex >= 0
    ? (currentIndex + delta + focusable.length) % focusable.length
    : (delta >= 0 ? 0 : focusable.length - 1);

  focusable[nextIndex]?.focus();
  return true;
}

export function trapFocusWithin(container: HTMLElement, event: KeyboardEvent): void {
  if (event.key !== 'Tab') {
    return;
  }

  const focusable = getFocusableElements(container);
  if (focusable.length === 0) {
    event.preventDefault();
    container.focus();
    return;
  }

  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const activeInside = activeElement ? container.contains(activeElement) : false;

  if (event.shiftKey) {
    if (!activeInside || activeElement === first) {
      event.preventDefault();
      last.focus();
    }
    return;
  }

  if (!activeInside || activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}