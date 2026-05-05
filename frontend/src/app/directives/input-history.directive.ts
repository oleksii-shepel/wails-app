import {
  Directive,
  ElementRef,
  HostListener,
  Inject,
  Input,
  NgZone,
  OnChanges,
  OnDestroy,
  OnInit,
  Optional,
  Renderer2,
  SimpleChanges,
} from '@angular/core';
import { NgForm } from '@angular/forms';
import { InputHistoryService } from '../services/input-history.service';

let datalistCounter = 0;

/**
 * Wires a text input to the InputHistoryService so that previously entered
 * values are offered as a native `<datalist>` autocomplete and the most
 * recent value is committed to history when the field is committed
 * (blur / Enter / form submit).
 *
 * Usage:
 *   <input type="text" [appInputHistory]="'remote.host'" [(ngModel)]="host" />
 *
 * If no key is provided, the input's `name` or `id` attribute is used.
 */
@Directive({
  selector: 'input[appInputHistory], textarea[appInputHistory]',
  standalone: true,
  exportAs: 'inputHistory',
})
export class InputHistoryDirective implements OnInit, OnChanges, OnDestroy {
  @Input('appInputHistory') key: string | null | undefined = null;

  /** When false the directive becomes a no-op without un-mounting it. */
  @Input() appInputHistoryEnabled = true;

  /** Minimum trimmed length required before a value is recorded. */
  @Input() appInputHistoryMinLength = 1;

  private readonly host: HTMLInputElement | HTMLTextAreaElement;
  private datalistEl: HTMLDataListElement | null = null;
  private datalistId = '';
  private formSubmitUnsubscribe: { unsubscribe(): void } | null = null;

  constructor(
    elementRef: ElementRef<HTMLInputElement | HTMLTextAreaElement>,
    private readonly renderer: Renderer2,
    private readonly history: InputHistoryService,
    private readonly ngZone: NgZone,
    @Optional() @Inject(NgForm) private readonly parentForm: NgForm | null,
  ) {
    this.host = elementRef.nativeElement;
  }

  ngOnInit(): void {
    if (!this.host.hasAttribute('autocomplete')) {
      this.renderer.setAttribute(this.host, 'autocomplete', 'off');
    }
    this.ensureDatalist();
    this.refreshOptions();

    if (this.parentForm) {
      this.formSubmitUnsubscribe = this.parentForm.ngSubmit.subscribe(() => this.commit());
    }
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['key'] || changes['appInputHistoryEnabled']) {
      this.refreshOptions();
    }
  }

  ngOnDestroy(): void {
    this.formSubmitUnsubscribe?.unsubscribe();
    this.formSubmitUnsubscribe = null;
    if (this.datalistEl?.parentNode) {
      this.datalistEl.parentNode.removeChild(this.datalistEl);
    }
    this.datalistEl = null;
  }

  /** Re-reads the stored values and rebuilds the datalist options. */
  refresh(): void {
    this.refreshOptions();
  }

  /** Manually push the current value to history (e.g. before programmatic submit). */
  commit(): void {
    const resolved = this.resolveKey();
    if (!resolved || !this.appInputHistoryEnabled) return;
    const value = (this.host.value ?? '').trim();
    if (value.length < this.appInputHistoryMinLength) return;
    this.history.push(resolved, value);
    this.refreshOptions();
  }

  @HostListener('change')
  onChange(): void {
    this.commit();
  }

  @HostListener('blur')
  onBlur(): void {
    this.commit();
  }

  @HostListener('keydown.enter')
  onEnter(): void {
    this.commit();
  }

  @HostListener('focus')
  onFocus(): void {
    this.refreshOptions();
  }

  private resolveKey(): string {
    if (this.key && this.key.trim()) return this.key.trim();
    return this.host.getAttribute('name') || this.host.id || '';
  }

  private ensureDatalist(): void {
    if (this.datalistEl) return;
    this.datalistId = `app-input-history-${++datalistCounter}`;
    const datalist = this.renderer.createElement('datalist') as HTMLDataListElement;
    datalist.id = this.datalistId;
    if (this.host.parentNode) {
      this.renderer.insertBefore(this.host.parentNode, datalist, this.host.nextSibling);
    } else {
      this.renderer.appendChild(document.body, datalist);
    }
    this.renderer.setAttribute(this.host, 'list', this.datalistId);
    this.datalistEl = datalist;
  }

  private refreshOptions(): void {
    if (!this.datalistEl) return;
    const resolved = this.resolveKey();
    const items = this.appInputHistoryEnabled && resolved ? this.history.get(resolved) : [];

    while (this.datalistEl.firstChild) {
      this.renderer.removeChild(this.datalistEl, this.datalistEl.firstChild);
    }
    this.ngZone.runOutsideAngular(() => {
      for (const item of items) {
        const option = this.renderer.createElement('option') as HTMLOptionElement;
        option.value = item;
        this.renderer.appendChild(this.datalistEl!, option);
      }
    });
  }
}
