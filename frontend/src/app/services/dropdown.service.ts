import { Injectable, OnDestroy } from '@angular/core';

/**
 * DropdownService — centralized manager for all dropdown open/close logic.
 *
 * ROOT CAUSE IT SOLVES:
 *   The classic race condition:
 *   1. User clicks a menu item
 *   2. `document:click` fires and closes the dropdown (isOpen = false)
 *   3. `*ngIf="isOpen"` destroys the DOM
 *   4. The item's `(click)` handler never fires — element is gone
 *
 * HOW IT WORKS:
 *   - Uses a `mousedown` listener on `document` (fires BEFORE click, blur, focus)
 *   - When mousedown fires, checks if the target is inside any registered dropdown
 *   - If inside → marks that dropdown as "interaction pending" (do NOT close)
 *   - The dropdown's own close logic checks this flag before closing
 *   - After the click fully resolves, the flag is cleared
 *
 * USAGE:
 *   1. Inject DropdownService into your component
 *   2. In your open logic: this.dropdownService.register(this.triggerEl, this.menuEl, () => this.close())
 *   3. In your close logic: if (this.dropdownService.isInteractionPending(id)) return;
 *   4. On destroy: this.dropdownService.unregister(id)
 *
 *   OR use the companion `appDropdown` directive for zero-boilerplate usage.
 */
@Injectable({ providedIn: 'root' })
export class DropdownService implements OnDestroy {
  private dropdowns = new Map<string, {
    trigger: Element;
    menu: Element;
    close: () => void;
  }>();

  private pendingId: string | null = null;
  private idCounter = 0;

  constructor() {
    // Use capture: true + mousedown so we run BEFORE any component sees the event
    document.addEventListener('mousedown', this.onGlobalMousedown, { capture: true });
    document.addEventListener('click', this.onGlobalClick, { capture: true });
  }

  ngOnDestroy(): void {
    document.removeEventListener('mousedown', this.onGlobalMousedown, { capture: true } as any);
    document.removeEventListener('click', this.onGlobalClick, { capture: true } as any);
  }

  /**
   * Register a dropdown. Returns a unique ID for this registration.
   *
   * @param trigger  The button/anchor that opens the dropdown
   * @param menu     The dropdown panel element
   * @param close    Callback to close this dropdown
   */
  register(trigger: Element, menu: Element, close: () => void): string {
    const id = `dd-${++this.idCounter}`;
    this.dropdowns.set(id, { trigger, menu, close });
    return id;
  }

  /** Update references after DOM changes (e.g. after *ngIf re-creates menu) */
  update(id: string, trigger: Element, menu: Element): void {
    const entry = this.dropdowns.get(id);
    if (entry) {
      entry.trigger = trigger;
      entry.menu = menu;
    }
  }

  unregister(id: string): void {
    this.dropdowns.delete(id);
    if (this.pendingId === id) this.pendingId = null;
  }

  /**
   * True if the user is currently clicking INSIDE this dropdown's menu.
   * Your close() logic should bail out if this returns true.
   */
  isInteractionPending(id: string): boolean {
    return this.pendingId === id;
  }

  /**
   * Close all other open dropdowns (pass the id of the one being opened).
   * Call this when opening a dropdown so others close.
   */
  closeOthers(exceptId: string): void {
    this.dropdowns.forEach((entry, id) => {
      if (id !== exceptId) entry.close();
    });
  }

  private onGlobalMousedown = (event: MouseEvent): void => {
    const target = event.target as Node;
    this.pendingId = null;

    for (const [id, { trigger, menu, close }] of this.dropdowns) {
      const inTrigger = trigger.contains(target);
      const inMenu = menu.contains(target);

      if (inMenu) {
        // User is clicking inside a menu — mark as pending so close() skips
        this.pendingId = id;
        return; // don't close anything
      }

      if (!inTrigger && !inMenu) {
        // Click is fully outside this dropdown — close it
        close();
      }
    }
  };

  private onGlobalClick = (_event: MouseEvent): void => {
    // Clear the pending flag after the full click cycle completes
    // Use a microtask so item (click) handlers still fire first
    Promise.resolve().then(() => { this.pendingId = null; });
  };
}