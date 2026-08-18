import { Injectable } from '@angular/core';

export interface ModalRegistration {
  readonly id: string;
  readonly element: HTMLElement;
  readonly dismiss: () => void;
  readonly dismissible: () => boolean;
}

@Injectable({ providedIn: 'root' })
export class ModalStackService {
  private readonly stack: ModalRegistration[] = [];

  register(registration: ModalRegistration): () => void {
    this.unregister(registration.id);
    this.stack.push(registration);
    return () => this.unregister(registration.id);
  }

  bringToFront(id: string): void {
    const index = this.stack.findIndex(item => item.id === id);
    if (index < 0 || index === this.stack.length - 1) return;
    const [registration] = this.stack.splice(index, 1);
    this.stack.push(registration);
  }

  dismissTopmost(): boolean {
    this.prune();

    for (let index = this.stack.length - 1; index >= 0; index -= 1) {
      const registration = this.stack[index];
      if (!registration.element.isConnected) continue;
      if (!registration.dismissible()) return false;

      registration.dismiss();
      return true;
    }

    return false;
  }

  topmost(): ModalRegistration | null {
    this.prune();
    return this.stack[this.stack.length - 1] ?? null;
  }

  private unregister(id: string): void {
    const index = this.stack.findIndex(item => item.id === id);
    if (index >= 0) this.stack.splice(index, 1);
  }

  private prune(): void {
    for (let index = this.stack.length - 1; index >= 0; index -= 1) {
      if (!this.stack[index].element.isConnected) {
        this.stack.splice(index, 1);
      }
    }
  }
}
