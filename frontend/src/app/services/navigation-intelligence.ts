export interface NavigationCandidate {
  readonly id: string;
  readonly label: string;
  readonly path: string;
  readonly kind: string;
  readonly accessCount?: number;
  readonly timestamp?: number;
  readonly pinned?: boolean;
  readonly current?: boolean;
}
export interface RankedNavigationCandidate<T extends NavigationCandidate> {
  readonly item: T; readonly score: number; readonly reasons: readonly string[];
}
const DAY = 86_400_000;
function segments(path: string): string[] {
  return (path ?? '').replace(/\\/g, '/').split('/').map(x => x.trim().toLowerCase()).filter(Boolean);
}
export function navigationAffinity(path: string, currentPath: string): number {
  const a=segments(path), b=segments(currentPath); let depth=0;
  while(depth<a.length && depth<b.length && a[depth]===b[depth]) depth++;
  return depth ? Math.min(18,(depth/Math.max(1,a.length))*18) : 0;
}
export function rankNavigationCandidates<T extends NavigationCandidate>(
  candidates: readonly T[], currentPath: string, now=Date.now(),
): RankedNavigationCandidate<T>[] {
  return candidates.map(item=>{
    let score=0; const reasons:string[]=[];
    if(item.pinned){score+=44;reasons.push('pinned')}
    const count=Math.max(0,item.accessCount??0);
    if(count){score+=Math.min(34,Math.log2(count+1)*8);reasons.push('frequent')}
    if(item.timestamp){
      const recency=Math.max(0,28-(Math.max(0,now-item.timestamp)/DAY)*4);
      score+=recency;if(recency>=8)reasons.push('recent');
    }
    const affinity=navigationAffinity(item.path,currentPath);
    if(affinity){score+=affinity;reasons.push('nearby')}
    if(item.current)score-=1000;
    return {item,score,reasons};
  }).filter(x=>x.score>-100).sort((a,b)=>b.score-a.score||a.item.label.localeCompare(b.item.label));
}
export function fuzzyPathScore(query:string,label:string,path:string):number{
  const needle=query.trim().toLowerCase(); if(!needle)return 0;
  const haystack=`${label} ${path}`.toLowerCase();
  if(haystack.startsWith(needle))return 120;if(haystack.includes(needle))return 90;
  let cursor=0,streak=0,score=0;
  for(const char of needle){const index=haystack.indexOf(char,cursor);if(index<0)return -1;streak=index===cursor?streak+1:0;score+=4+Math.min(10,streak*2);cursor=index+1}
  return score-Math.min(30,Math.floor(haystack.length/16));
}
