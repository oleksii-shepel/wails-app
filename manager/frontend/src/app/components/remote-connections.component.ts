import {ChangeDetectionStrategy,ChangeDetectorRef,Component,EventEmitter,OnDestroy,OnInit,Output} from '@angular/core';
import {CommonModule} from '@angular/common';
import { writeClipboardText } from '../utils/clipboard-text';
import {createSubject,takeUntil} from '@epikodelabs/streamix';
import {contracts} from '../../../wailsjs/go/models';
import {ConfirmDialogService} from '../services/confirm-dialog.service';
import {InputEngineService} from '../services/input-engine.service';
import type {InputBindingDefinition} from '../services/input-system.types';
import {tryInjectElementRef,tryInjectModalStack} from '../services/modal-stack.service';
import {RemoteConnectionsService,type RemoteConnectionSnapshot} from '../services/remote-connections.service';
import {RemoteConnectDialogService} from '../services/remote-connect-dialog.service';
const CONTEXT_ID='overlay.RemoteConnections';
const OWNER_ID=CONTEXT_ID;
const COMMAND={close:'reassembly.remoteConnections.close',previous:'reassembly.remoteConnections.previous',next:'reassembly.remoteConnections.next',open:'reassembly.remoteConnections.open',connect:'reassembly.remoteConnections.connect',disconnect:'reassembly.remoteConnections.disconnect',refresh:'reassembly.remoteConnections.refresh'} as const;
const BINDINGS:readonly InputBindingDefinition[]=[
{id:'reassembly.remoteConnections.escape',commandId:COMMAND.close,contextId:CONTEXT_ID,source:'default',description:'Close remote connections',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'escape'}]}}},
{id:'reassembly.remoteConnections.up',commandId:COMMAND.previous,contextId:CONTEXT_ID,source:'default',description:'Previous remote connection',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'arrowup'}]}}},
{id:'reassembly.remoteConnections.down',commandId:COMMAND.next,contextId:CONTEXT_ID,source:'default',description:'Next remote connection',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'arrowdown'}]}}},
{id:'reassembly.remoteConnections.enter',commandId:COMMAND.open,contextId:CONTEXT_ID,source:'default',description:'Open remote connection',category:'Remote',when:'!typing',trigger:{device:'keyboard',chord:{steps:[{key:'enter'}]}}},
{id:'reassembly.remoteConnections.delete',commandId:COMMAND.disconnect,contextId:CONTEXT_ID,source:'default',description:'Disconnect remote connection',category:'Remote',when:'!typing',trigger:{device:'keyboard',chord:{steps:[{key:'delete'}]}}},
{id:'reassembly.remoteConnections.insert',commandId:COMMAND.connect,contextId:CONTEXT_ID,source:'default',description:'Connect remote',category:'Remote',when:'!typing',trigger:{device:'keyboard',chord:{steps:[{key:'insert'}]}}},
{id:'reassembly.remoteConnections.f5',commandId:COMMAND.refresh,contextId:CONTEXT_ID,source:'default',description:'Refresh remote connections',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'f5'}]}}},
];

@Component({selector:'app-remote-connections',standalone:true,imports: [CommonModule],changeDetection:ChangeDetectionStrategy.OnPush,
template:`<div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="renderVisible" [class.backdrop--closing]="isClosing" [class.dialog-group--closing]="isClosing" (mousedown)="close()" (animationend)="onBackdropAnimationEnd($event)"><section class="panel palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Remote connections" (mousedown)="$event.stopPropagation()"
        tabindex="-1" (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
<header class="palette-dialog-header"><div class="palette-dialog-title"><strong>Remote connections</strong><span class="palette-dialog-subtitle" role="status">{{matchingMounts.length}} of {{state.mounts.length}}</span></div><div class="header-actions"><button type="button" class="palette-dialog-button" (click)="connect()">Connect…</button><button type="button" class="palette-dialog-button" (click)="refresh()" [disabled]="state.loading">{{state.loading?'Refreshing…':'Refresh'}}</button><button type="button" class="dialog-close" aria-label="Close dialog" title="Close" (click)="close()"> × </button></div></header>
<div class="palette-dialog-toolbar">
  <input class="palette-dialog-field remote-filter" type="search" placeholder="Filter connections…" aria-label="Filter remote connections"
    [value]="query" (input)="onQueryInput($event)" (keydown.enter)="openSelectedFromFilter($event)" />
  <button type="button" class="palette-dialog-button" [disabled]="!query" (click)="query = '';
copyNotice = '';
disconnecting = false; selectedIndex = 0">Clear filter</button>
</div>
<div class="error" *ngIf="state.error" role="alert">{{state.error}}</div>
@let matching = matchingMounts;
<div class="list palette-dialog-body palette-dialog-list" role="listbox" aria-label="Remote connections" [attr.aria-busy]="state.loading">
<button type="button" class="mount palette-dialog-item" role="option" *ngFor="let mount of matching;let i=index;trackBy:trackMount" [class.selected]="i===selectedIndex" [class.is-selected]="i===selectedIndex" [attr.aria-selected]="i===selectedIndex" [attr.aria-posinset]="i + 1" [attr.aria-setsize]="matching.length" [attr.tabindex]="i===selectedIndex ? 0 : -1" (mouseenter)="selectedIndex=i" (click)="openMount(mount)">
<span class="icon" aria-hidden="true">☁</span><span class="details"><strong>{{label(mount)}}</strong><small [title]="uri(mount)">{{uri(mount)}}</small></span><span class="mount-status"><span class="read-only" *ngIf="mount.readOnly">Read only</span><span class="protocol">{{protocol(mount)}}</span></span>
</button>
<div class="empty palette-dialog-empty" *ngIf="!state.loading&&!state.error&&state.mounts.length===0" role="status"><strong>No remote connections.</strong><span>Choose Connect or press Insert to add one.</span></div>
<div class="empty palette-dialog-empty" *ngIf="!state.loading && state.mounts.length && !matching.length" role="status">No matching connections.</div>
</div>
<footer class="palette-dialog-footer"><div class="palette-dialog-footer-actions"><button type="button" class="palette-dialog-button" [disabled]="!selectedMount" (click)="copySelectedLocation()">Copy Location</button><button type="button" class="palette-dialog-button palette-dialog-button--danger" [disabled]="!selectedMount || disconnecting" (click)="disconnectSelected()">Disconnect</button></div><span class="palette-dialog-meta" role="status">{{copyNotice}}</span><span *ngIf="state.refreshedAt" class="palette-dialog-meta" [title]="state.refreshedAt | date:'medium'">Updated {{state.refreshedAt | date:'shortTime'}}</span><span class="palette-dialog-footnote">↑ ↓ select · Enter open · Insert connect · Del disconnect · F5 refresh</span></footer>
</section></div>`,
styles:[`:host{position:fixed;inset:0;z-index:9550;pointer-events:none}.backdrop{pointer-events:auto}.backdrop--closing{pointer-events:none}.panel{width:min(560px,calc(100vw - 24px));max-height:min(650px,calc(100vh - 40px));display:flex;flex-direction:column}.header-actions{display:flex;flex-wrap:wrap;gap:5px;align-items:center}.palette-dialog-header,.palette-dialog-footer{flex-wrap:wrap}.remote-filter{flex:1}.list{min-height:120px;overflow:auto}.mount{display:grid;grid-template-columns:28px minmax(0,1fr) auto;gap:9px;align-items:center;width:100%;min-height:48px;padding:6px 8px;text-align:left}.mount.selected{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}.details{display:grid;min-width:0}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small,.protocol{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.mount-status{display:grid;justify-items:end;gap:2px}.protocol{text-transform:uppercase;font-size:9px}.read-only{color:var(--vsc-warning-foreground,var(--vsc-warning));font-size:9px}.empty{display:grid;place-items:center;align-content:center;gap:4px;min-height:130px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.error{padding:7px 10px;color:var(--vsc-error-foreground,var(--vsc-error))}`]})
export class RemoteConnectionsComponent implements OnInit,OnDestroy{
@Output() navigate=new EventEmitter<string>(); visible=false; renderVisible=false; isClosing=false; state:RemoteConnectionSnapshot={mounts:[],loading:false,error:'',refreshedAt:0}; selectedIndex=0;
private readonly destroy$=createSubject<void>();
private readonly modalStack=tryInjectModalStack();
private readonly hostElement=tryInjectElementRef();
private unregisterModal:(()=>void)|null=null;
constructor(private readonly connections:RemoteConnectionsService,private readonly connectDialog:RemoteConnectDialogService,private readonly confirm:ConfirmDialogService,private readonly input:InputEngineService,private readonly cdr:ChangeDetectorRef){}
query = '';
copyNotice = '';
disconnecting = false;
get matchingMounts(): contracts.RemoteMount[] {
  const query = this.query.trim().toLowerCase();
  if (!query) return this.state.mounts;
  return this.state.mounts.filter(mount =>
    [this.label(mount), this.protocol(mount), this.uri(mount)].some(value => value.toLowerCase().includes(query)),
  );
}
get selectedMount(): contracts.RemoteMount | null { return this.matchingMounts[this.selectedIndex] ?? null; }
openSelectedFromFilter(event: Event): void {
  event.preventDefault();
  event.stopPropagation();
  if (this.selectedMount) this.openMount(this.selectedMount);
}
onQueryInput(event: Event): void {
  this.query = (event.target as HTMLInputElement).value;
  this.selectedIndex = 0;
  this.cdr.markForCheck();
}
ngOnInit():void{this.input.registerContext({id:CONTEXT_ID,kind:'overlay',priority:420, exclusive: true,parentId:'global',description:'Remote connections'});const d=(
      id:string,
      title:string,
      execute:()=>void|Promise<void>,
      isEnabled?:()=>boolean,
    )=>this.input.registerCommandDescriptor(
      {id,title,category:'Remote',execute,isEnabled},
      OWNER_ID,
    );d(COMMAND.close,'Close Remote Connections',()=>this.close());d(COMMAND.previous,'Previous Remote Connection',()=>this.move(-1));d(COMMAND.next,'Next Remote Connection',()=>this.move(1));d(COMMAND.open,'Open Remote Connection',()=>{const m=this.selectedMount;if(m)this.openMount(m)},()=>!!this.selectedMount);d(COMMAND.connect,'Connect Remote',()=>this.connect());d(COMMAND.disconnect,'Disconnect Remote',()=>this.disconnectSelected(),()=>!!this.selectedMount&&!this.disconnecting);d(COMMAND.refresh,'Refresh Remote Connections',()=>this.refresh());for(const b of BINDINGS)this.input.registerBinding(b);this.connections.state$.pipe(takeUntil(this.destroy$)).subscribe(state => {
      const selected = this.selectedMount;
      this.state = state;
      const matching = this.matchingMounts;
      const index = selected ? matching.findIndex(mount => this.trackMount(0, mount) === this.trackMount(0, selected)) : -1;
      this.selectedIndex = index >= 0 ? index : Math.min(this.selectedIndex, Math.max(0, matching.length - 1));
      this.cdr.markForCheck();
    })}
ngOnDestroy():void{this.unregisterModal?.();this.unregisterModal=null;this.input.deactivateContext(CONTEXT_ID);for(const b of BINDINGS)this.input.unregisterBinding(b.id);for(const id of Object.values(COMMAND))this.input.unregisterCommandDescriptor(id,OWNER_ID);this.destroy$.next();this.destroy$.complete()}
open():void{if(!this.visible){this.copyNotice='';this.query='';this.selectedIndex=0;}this.visible=true;this.renderVisible=true;this.isClosing=false;this.input.activateContext(CONTEXT_ID);if(this.modalStack&&this.hostElement&&!this.unregisterModal){this.unregisterModal=this.modalStack.register({id:'remote-connections',element:this.hostElement.nativeElement,contextId:CONTEXT_ID,dismiss:()=>this.close(),dismissible:()=>this.renderVisible&&!this.isClosing});}this.cdr.markForCheck();void this.refresh()}
close():void{if(!this.renderVisible||this.isClosing)return;this.visible=false;this.isClosing=true;this.unregisterModal?.();this.unregisterModal=null;this.input.deactivateContext(CONTEXT_ID);this.cdr.markForCheck()}
async copySelectedLocation(): Promise<void> {
  const mount = this.selectedMount;
  const location = mount && this.uri(mount);
  if (!location) return;
  try {
    await writeClipboardText(location);
    this.copyNotice = 'Location copied.';
  } catch {
    this.copyNotice = 'Clipboard unavailable.';
  }
  this.cdr.markForCheck();
}
async refresh():Promise<void>{await this.connections.refresh()}
async connect():Promise<void>{const mount=await this.connectDialog.openDialog();if(!mount)return;await this.connections.refresh();const uri=this.connections.rootUri(mount);if(uri){this.navigate.emit(uri);this.close()}}
async disconnectSelected(): Promise<void> {
  const mount = this.selectedMount;
  if (!mount || this.disconnecting) return;
  this.disconnecting = true;
  this.cdr.markForCheck();
  try {
    const confirmed = await this.confirm.confirm({
      title: 'Disconnect Remote',
      message: `Disconnect "${this.connections.label(mount)}"?`,
      confirmText: 'Disconnect', danger: true,
    });
    if (confirmed) await this.connections.disconnect(mount);
  } finally {
    this.disconnecting = false;
    this.cdr.markForCheck();
  }
}
openMount(mount:contracts.RemoteMount):void{const uri=this.connections.rootUri(mount);if(uri){this.navigate.emit(uri);this.close()}}
move(delta:-1|1):void{const count=this.matchingMounts.length;if(count){this.selectedIndex=(this.selectedIndex+delta+count)%count;this.cdr.markForCheck()}}
label(m:contracts.RemoteMount):string{return this.connections.label(m)} uri(m:contracts.RemoteMount):string{return this.connections.rootUri(m)} protocol(m:contracts.RemoteMount):string{return this.connections.protocol(m)||'remote'} trackMount(_i:number,m:contracts.RemoteMount):string{return this.connections.mountPoint(m)||this.connections.rootUri(m)}
onBackdropAnimationEnd(event: AnimationEvent): void { if (event.target !== event.currentTarget || !this.isClosing) return; this.renderVisible=false; this.isClosing=false; this.cdr.markForCheck() }
}