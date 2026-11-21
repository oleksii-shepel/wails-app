/** Write text using the browser clipboard or the native desktop clipboard. */
export async function writeClipboardText(text: string): Promise<void> {
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      await navigator.clipboard.writeText(text);
      return;
    }
  } catch {
    // The browser can deny clipboard access inside the native WebView.
  }

  if (typeof window !== 'undefined' && await window.runtime?.ClipboardSetText?.(text)) {
    return;
  }

  throw new Error('Clipboard unavailable.');
}
