import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export interface RegistryValueEditRequest {
  path: string;
  name: string;
  type: string;
  data: string;
}

export interface RegistryValueEditResult {
  type: string;
  data: string;
}

interface ActiveRegistryValueEdit {
  request: RegistryValueEditRequest;
  resolve: (result: RegistryValueEditResult | null) => void;
}

@Injectable({ providedIn: 'root' })
export class RegistryValueEditorService {
  readonly active$ = createBehaviorSubject<ActiveRegistryValueEdit | null>(null);

  edit(request: RegistryValueEditRequest): Promise<RegistryValueEditResult | null> {
    // Registry edit is intentionally single-instance. Resolve any stale request
    // as cancelled instead of stacking invisible prompt surfaces.
    const active = this.active$.value;
    if (active) {
      active.resolve(null);
    }

    return new Promise(resolve => {
      this.active$.next({ request, resolve });
    });
  }

  resolve(result: RegistryValueEditResult | null): void {
    const active = this.active$.value;
    if (!active) return;
    this.active$.next(null);
    active.resolve(result);
  }
}
