import { Injectable } from '@angular/core';
import { Subject } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class TrashDialogService {
  readonly openRequested$ = new Subject<void>();
  readonly mutated$ = new Subject<void>();

  open(): void {
    this.openRequested$.next();
  }

  notifyMutated(): void {
    this.mutated$.next();
  }
}
