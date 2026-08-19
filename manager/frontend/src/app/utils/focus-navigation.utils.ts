const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[contenteditable="true"]',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

function isFocusable(element: HTMLElement): boolean {
  if (!element.isConnected || element.tabIndex < 0) return false;
  if (element.hasAttribute('disabled') || element.getAttribute('aria-disabled') === 'true') return false;
  if (element.closest('[hidden], [inert], [aria-hidden="true"]')) return false;
  const style = getComputedStyle(element);
  return style.display !== 'none' &&
    style.visibility !== 'hidden' &&
    style.visibility !== 'collapse' &&
    element.getClientRects().length > 0;
}

export function moveFocusWithin(
  container: HTMLElement,
  delta: number,
  selector?: string,
): boolean {
  const candidates = Array.from(
    container.querySelectorAll<HTMLElement>(selector || FOCUSABLE_SELECTOR),
  ).filter(isFocusable);

  if (!candidates.length) {
    container.focus();
    return false;
  }

  const active = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : null;
  const current = active && container.contains(active)
    ? candidates.indexOf(active)
    : -1;
  const next = current >= 0
    ? (current + delta + candidates.length) % candidates.length
    : (delta >= 0 ? 0 : candidates.length - 1);

  candidates[next]?.focus();
  return true;
}
