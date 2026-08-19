import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export interface RegistryKeyEditRequest {
  title: string;
  name: string;
  confirmText: string;
}

interface ActiveRegistryKeyEdit {
  request: RegistryKeyEditRequest;
  resolve: (result: string | null) => void;
}

@Injectable({ providedIn: 'root' })
export class RegistryKeyEditorService {
  readonly active$ = createBehaviorSubject<ActiveRegistryKeyEdit | null>(null);

  open(request: RegistryKeyEditRequest): Promise<string | null> {
    const active = this.active$.value;
    if (active) {
      active.resolve(null);
    }

    return new Promise(resolve => {
      this.active$.next({ request, resolve });
    });
  }

  resolve(result: string | null): void {
    const active = this.active$.value;
    if (!active) return;
    this.active$.next(null);
    active.resolve(result);
  }
}
