import {ChangeDetectionStrategy,ChangeDetectorRef,Component,EventEmitter,OnDestroy,OnInit,Output} from '@angular/core';
import {CommonModule} from '@angular/common';
import {createSubject,takeUntil} from '@epikodelabs/streamix';
import {contracts} from '../../../wailsjs/go/models';
import {ConfirmDialogService} from '../services/confirm-dialog.service';
import {InputEngineService} from '../services/input-engine.service';
import type {InputBindingDefinition} from '../services/input-system.types';
import {RemoteConnectionsService,type RemoteConnectionSnapshot} from '../services/remote-connections.service';
import {RemoteConnectDialogService} from '../services/remote-connect-dialog.service';
import { ReassemblyModalSurfaceDirective } from './reassembly-modal-surface.directive';
const CONTEXT_ID='overlay.reassemblyRemoteConnections';
const OWNER_ID=CONTEXT_ID;
const COMMAND={close:'reassembly.remoteConnections.close',previous:'reassembly.remoteConnections.previous',next:'reassembly.remoteConnections.next',open:'reassembly.remoteConnections.open',connect:'reassembly.remoteConnections.connect',disconnect:'reassembly.remoteConnections.disconnect',refresh:'reassembly.remoteConnections.refresh'} as const;
const BINDINGS:readonly InputBindingDefinition[]=[
{id:'reassembly.remoteConnections.escape',commandId:COMMAND.close,contextId:CONTEXT_ID,source:'default',description:'Close remote connections',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'escape'}]}}},
{id:'reassembly.remoteConnections.up',commandId:COMMAND.previous,contextId:CONTEXT_ID,source:'default',description:'Previous remote connection',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'arrowup'}]}}},
{id:'reassembly.remoteConnections.down',commandId:COMMAND.next,contextId:CONTEXT_ID,source:'default',description:'Next remote connection',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'arrowdown'}]}}},
{id:'reassembly.remoteConnections.enter',commandId:COMMAND.open,contextId:CONTEXT_ID,source:'default',description:'Open remote connection',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'enter'}]}}},
{id:'reassembly.remoteConnections.delete',commandId:COMMAND.disconnect,contextId:CONTEXT_ID,source:'default',description:'Disconnect remote connection',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'delete'}]}}},
{id:'reassembly.remoteConnections.insert',commandId:COMMAND.connect,contextId:CONTEXT_ID,source:'default',description:'Connect remote',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'insert'}]}}},
{id:'reassembly.remoteConnections.f5',commandId:COMMAND.refresh,contextId:CONTEXT_ID,source:'default',description:'Refresh remote connections',category:'Remote',trigger:{device:'keyboard',chord:{steps:[{key:'f5'}]}}},
];

@Component({selector:'app-reassembly-remote-connections',standalone:true,imports: [CommonModule, ReassemblyModalSurfaceDirective],changeDetection:ChangeDetectionStrategy.OnPush,
template:`<div class="backdrop" *ngIf="visible" (mousedown)="close()"><section class="panel" role="dialog" aria-modal="true" aria-label="Remote connections" (mousedown)="$event.stopPropagation()"
        reassemblyModalSurface
        reassemblyModalInitial="[aria-selected='true']"
        tabindex="-1">
<header><div><strong>Remote connections</strong><span>{{state.mounts.length}}</span></div><div class="header-actions"><button type="button" (click)="connect()">+ Connect</button><button type="button" (click)="refresh()" [disabled]="state.loading">{{state.loading?'Refreshing…':'Refresh'}}</button></div></header>
<div class="error" *ngIf="state.error" role="alert">{{state.error}}</div>
<div class="list" role="listbox" aria-label="Remote connections" [attr.aria-busy]="state.loading">
<button type="button" class="mount" role="option" *ngFor="let mount of state.mounts;let i=index;trackBy:trackMount" [class.selected]="i===selectedIndex" [attr.aria-selected]="i===selectedIndex" [attr.aria-posinset]="i + 1" [attr.aria-setsize]="state.mounts.length" [attr.tabindex]="i===selectedIndex ? 0 : -1" (mouseenter)="selectedIndex=i" (click)="openMount(mount)">
<span class="icon">☁</span><span class="details"><strong>{{label(mount)}}</strong><small>{{uri(mount)}}</small></span><span class="mount-status"><span class="read-only" *ngIf="mount.readOnly">Read only</span><span class="protocol">{{protocol(mount)}}</span></span>
</button>
<div class="empty" *ngIf="!state.loading&&!state.error&&state.mounts.length===0"><strong>No remote connections.</strong><span>Use + Connect or Insert to add one.</span></div>
</div>
<footer><button type="button" class="danger" [disabled]="!selectedMount" (click)="disconnectSelected()">Del&nbsp;&nbsp;Disconnect</button><span>↑ ↓ select · Enter open · Insert connect · F5 refresh · Esc close</span></footer>
</section></div>`,
styles:[`:host{position:fixed;inset:0;z-index:9550;pointer-events:none}.backdrop{position:fixed;inset:0;display:grid;place-items:start center;padding-top:min(12vh,110px);background:rgba(0,0,0,.28);pointer-events:auto}.panel{width:min(560px,calc(100vw - 24px));max-height:min(650px,calc(100vh - 40px));display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--vsc-border,#454545);border-radius:8px;background:var(--vsc-menu-background,#252526);color:var(--vsc-foreground,#ccc);box-shadow:0 18px 60px rgba(0,0,0,.48);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}header,footer{display:flex;align-items:center;gap:8px;padding:8px 10px}header{justify-content:space-between;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)}header>div:first-child{display:flex;gap:8px;align-items:baseline}.header-actions{display:flex;gap:5px}button{border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;cursor:pointer;font:inherit}button:hover:not(:disabled){background:var(--vsc-list-hover-background,#2a2d2e);border-color:var(--vsc-border,#454545)}button:disabled{opacity:.35}header button,footer button{min-height:28px;padding:4px 8px}.list{min-height:120px;overflow:auto;padding:4px}.mount{display:grid;grid-template-columns:28px minmax(0,1fr) auto;gap:9px;align-items:center;width:100%;min-height:48px;padding:6px 8px;text-align:left}.mount.selected{background:var(--vsc-list-hover-background,#2a2d2e)}.details{display:grid;min-width:0}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small,footer span,header span,.protocol{color:var(--vsc-description-foreground,#969696)}.mount-status{display:grid;justify-items:end;gap:2px}.protocol{text-transform:uppercase;font-size:9px}.read-only{color:var(--vsc-warning-foreground,#cca700);font-size:9px}.empty{display:grid;place-items:center;align-content:center;gap:4px;min-height:130px;color:var(--vsc-description-foreground,#969696)}.error{padding:7px 10px;color:var(--vsc-error-foreground,#f48771)}footer{border-top:1px solid var(--vsc-border-subtle,#2b2b2b)}footer span{margin-left:auto;font-size:9px}.danger{color:var(--vsc-error-foreground,#f48771)}`]})
export class ReassemblyRemoteConnectionsComponent implements OnInit,OnDestroy{
@Output() navigate=new EventEmitter<string>(); visible=false; state:RemoteConnectionSnapshot={mounts:[],loading:false,error:'',refreshedAt:0}; selectedIndex=0;
private readonly destroy$=createSubject<void>();
constructor(private readonly connections:RemoteConnectionsService,private readonly connectDialog:RemoteConnectDialogService,private readonly confirm:ConfirmDialogService,private readonly input:InputEngineService,private readonly cdr:ChangeDetectorRef){}
get selectedMount():contracts.RemoteMount|null{return this.state.mounts[this.selectedIndex]??null}
ngOnInit():void{this.input.registerContext({id:CONTEXT_ID,kind:'overlay',priority:420, exclusive: true,parentId:'global',description:'Remote connections'});const d=(
      id:string,
      title:string,
      execute:()=>void|Promise<void>,
      isEnabled?:()=>boolean,
    )=>this.input.registerCommandDescriptor(
      {id,title,category:'Remote',execute,isEnabled},
      OWNER_ID,
    );d(COMMAND.close,'Close Remote Connections',()=>this.close());d(COMMAND.previous,'Previous Remote Connection',()=>this.move(-1));d(COMMAND.next,'Next Remote Connection',()=>this.move(1));d(COMMAND.open,'Open Remote Connection',()=>{const m=this.selectedMount;if(m)this.openMount(m)},()=>!!this.selectedMount);d(COMMAND.connect,'Connect Remote',()=>this.connect());d(COMMAND.disconnect,'Disconnect Remote',()=>this.disconnectSelected(),()=>!!this.selectedMount);d(COMMAND.refresh,'Refresh Remote Connections',()=>this.refresh());for(const b of BINDINGS)this.input.registerBinding(b);this.connections.state$.pipe(takeUntil(this.destroy$)).subscribe(state=>{this.state=state;this.selectedIndex=Math.min(this.selectedIndex,Math.max(0,state.mounts.length-1));this.cdr.markForCheck()})}
ngOnDestroy():void{this.input.deactivateContext(CONTEXT_ID);for(const b of BINDINGS)this.input.unregisterBinding(b.id);for(const id of Object.values(COMMAND))this.input.unregisterCommandDescriptor(id,OWNER_ID);this.destroy$.next();this.destroy$.complete()}
open():void{this.visible=true;this.input.activateContext(CONTEXT_ID);this.cdr.markForCheck();void this.refresh()}
close():void{if(!this.visible)return;this.visible=false;this.input.deactivateContext(CONTEXT_ID);this.cdr.markForCheck()}
async refresh():Promise<void>{await this.connections.refresh()}
async connect():Promise<void>{const mount=await this.connectDialog.openDialog();if(!mount)return;await this.connections.refresh();const uri=this.connections.rootUri(mount);if(uri){this.navigate.emit(uri);this.close()}}
async disconnectSelected():Promise<void>{const mount=this.selectedMount;if(!mount)return;const ok=await this.confirm.confirm({title:'Disconnect Remote',message:`Disconnect "${this.connections.label(mount)}"?`,confirmText:'Disconnect',danger:true});if(ok)await this.connections.disconnect(mount)}
openMount(mount:contracts.RemoteMount):void{const uri=this.connections.rootUri(mount);if(uri){this.navigate.emit(uri);this.close()}}
move(delta:-1|1):void{const count=this.state.mounts.length;if(count){this.selectedIndex=(this.selectedIndex+delta+count)%count;this.cdr.markForCheck()}}
label(m:contracts.RemoteMount):string{return this.connections.label(m)} uri(m:contracts.RemoteMount):string{return this.connections.rootUri(m)} protocol(m:contracts.RemoteMount):string{return this.connections.protocol(m)||'remote'} trackMount(_i:number,m:contracts.RemoteMount):string{return this.connections.mountPoint(m)||this.connections.rootUri(m)}
}
