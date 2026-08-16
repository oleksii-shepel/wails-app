import { Injectable } from '@angular/core';
import { Subject } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class ReassemblyTrashDialogService {
  readonly openRequested$ = new Subject<void>();

  open(): void {
    this.openRequested$.next();
  }
}
