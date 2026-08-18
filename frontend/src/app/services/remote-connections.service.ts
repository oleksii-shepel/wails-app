import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { contracts } from '../../../wailsjs/go/models';
import { ApiService } from './api.service';
import type { SourceCapabilities } from './source-capabilities';
import { normalizeProviderCapabilities, UNKNOWN_PROVIDER_CAPABILITIES, type ProviderCapabilities } from './provider-capabilities';

export type RemoteHierarchy = 'share-path' | 'path-backed';

export interface RemoteConnectionSnapshot {
  mounts: contracts.RemoteMount[];
  loading: boolean;
  error: string;
  refreshedAt: number;
}
const EMPTY: RemoteConnectionSnapshot = { mounts: [], loading: false, error: '', refreshedAt: 0 };

@Injectable({ providedIn: 'root' })
export class RemoteConnectionsService {
  readonly state$ = createBehaviorSubject<RemoteConnectionSnapshot>(EMPTY);
  private generation = 0;
  constructor(private readonly api: ApiService) {}

  get mounts(): contracts.RemoteMount[] { return this.state$.value.mounts; }

  async refresh(): Promise<contracts.RemoteMount[]> {
    const generation = ++this.generation;
    this.patch({loading:true,error:''});
    try {
      const mounts = await this.api.remoteListConnections();
      if (generation !== this.generation) return this.mounts;
      const normalized=[...(mounts??[])].sort((a,b)=>this.label(a).localeCompare(this.label(b),undefined,{numeric:true,sensitivity:'base'}));
      this.state$.next({mounts:normalized,loading:false,error:'',refreshedAt:Date.now()});
      return normalized;
    } catch(error) {
      if (generation !== this.generation) return this.mounts;
      this.patch({loading:false,error:error instanceof Error?error.message:String(error)});
      return this.mounts;
    }
  }

  async disconnect(mountOrPoint: contracts.RemoteMount|string): Promise<boolean> {
    const mountPoint=typeof mountOrPoint==='string'?mountOrPoint.trim():String(mountOrPoint.mountPoint??'').trim();
    if(!mountPoint)return false;
    try { await this.api.remoteDisconnect(mountPoint); await this.refresh(); return true; }
    catch(error){ this.patch({error:error instanceof Error?error.message:String(error)}); return false; }
  }

  findByUri(uri:string):contracts.RemoteMount|null{
    const needle=this.normalizeUri(uri); if(!needle)return null;
    return this.mounts.find(mount=>{
      const root=this.normalizeUri(mount.publicPath??'');
      return !!root&&(needle===root||needle.startsWith(root.endsWith('/')?root:root+'/'));
    })??null;
  }

  rootUri(mount:contracts.RemoteMount):string{return String(mount.publicPath??'').trim()}
  mountPoint(mount:contracts.RemoteMount):string{return String(mount.mountPoint??'').trim()}
  label(mount:contracts.RemoteMount):string{return String(mount.name||mount.publicPath||mount.host||'Remote')}
  protocol(mount:contracts.RemoteMount):string{return String(mount.protocol??'').trim().toLocaleLowerCase()}

  isRemoteUri(uri:string):boolean{
    const scheme=/^([a-z][a-z0-9+.-]*):\/\//i.exec((uri??'').trim())?.[1]?.toLowerCase();
    return scheme==='smb'||scheme==='s3'||scheme==='gdrive'||scheme==='dropbox'||scheme==='onedrive';
  }

  hierarchyFor(uri:string):RemoteHierarchy{
    const scheme=/^([a-z][a-z0-9+.-]*):\/\//i.exec((uri??'').trim())?.[1]?.toLowerCase();
    return scheme==='smb'?'share-path':'path-backed';
  }

  providerCapabilitiesFor(uri:string):ProviderCapabilities{const mount=this.findByUri(uri);if(!mount)return UNKNOWN_PROVIDER_CAPABILITIES;return normalizeProviderCapabilities((mount as contracts.RemoteMount&{capabilities?:Partial<ProviderCapabilities>}).capabilities)}

  capabilitiesFor(uri:string,base:SourceCapabilities):SourceCapabilities{const mount=this.findByUri(uri);if(!mount)return base;const provider=this.providerCapabilitiesFor(uri);if(mount.readOnly)return {read:base.read,create:false,delete:false,rename:false,trash:false};return {...base,rename:base.rename&&provider.rename,trash:base.trash&&provider.trash}}

  private normalizeUri(value:string):string{return (value??'').trim().replace(/\/+$/,'').toLocaleLowerCase()}
  private patch(patch:Partial<RemoteConnectionSnapshot>):void{this.state$.next({...this.state$.value,...patch})}
}