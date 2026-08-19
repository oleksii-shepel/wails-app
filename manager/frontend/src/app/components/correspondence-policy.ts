export type CorrespondenceStatus =
  | 'equal'|'only-left'|'only-right'|'different'|'left-newer'|'right-newer'|'type-mismatch';

export function markerForSide(
  status: CorrespondenceStatus,
  side: 'left'|'right',
): 'equal'|'only-here'|'different'|'newer'|'older'|'type-mismatch' {
  switch (status) {
    case 'only-left': return side === 'left' ? 'only-here' : 'equal';
    case 'only-right': return side === 'right' ? 'only-here' : 'equal';
    case 'different': return 'different';
    case 'left-newer': return side === 'left' ? 'newer' : 'older';
    case 'right-newer': return side === 'right' ? 'newer' : 'older';
    case 'type-mismatch': return 'type-mismatch';
    default: return 'equal';
  }
}

export function nextDifferenceIndex(current:number,count:number,delta:-1|1):number {
  if (count <= 0) return -1;
  if (current < 0) return delta > 0 ? 0 : count - 1;
  return (current + delta + count) % count;
}