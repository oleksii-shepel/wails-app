export type PaneLocationRelation = 'same'|'left-inside-right'|'right-inside-left'|'different';

export interface PaneLocationAnalysis {
  readonly relation: PaneLocationRelation;
  readonly depthDelta: number;
}

function slash(value: string): string {
  return (value ?? '').trim().replace(/\\/g, '/').replace(/\/+/g, '/');
}

function split(value: string): { prefix: string; segments: string[] } {
  const normalized = slash(value).replace(/\/$/, '');
  const scheme = /^([a-z][a-z0-9+.-]*:\/\/[^/]*)(?:\/(.*))?$/i.exec(normalized);
  if (scheme) return { prefix: scheme[1].toLowerCase(), segments: (scheme[2] ?? '').split('/').filter(Boolean) };

  const drive = /^([a-z]:)(?:\/(.*))?$/i.exec(normalized);
  if (drive) return { prefix: drive[1].toLowerCase(), segments: (drive[2] ?? '').split('/').filter(Boolean) };

  return { prefix: normalized.startsWith('/') ? '/' : '', segments: normalized.split('/').filter(Boolean) };
}

const sameSegment = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();

export function samePaneLocation(left: string, right: string): boolean {
  const a = split(left), b = split(right);
  return a.prefix === b.prefix &&
    a.segments.length === b.segments.length &&
    a.segments.every((x, i) => sameSegment(x, b.segments[i]));
}

export function analyzePaneLocations(left: string, right: string): PaneLocationAnalysis {
  const a = split(left), b = split(right);
  if (samePaneLocation(left, right)) return { relation:'same', depthDelta:0 };
  if (a.prefix !== b.prefix) return { relation:'different', depthDelta:0 };

  const common = Math.min(a.segments.length, b.segments.length);
  let i = 0;
  while (i < common && sameSegment(a.segments[i], b.segments[i])) i++;
  if (i !== common) return { relation:'different', depthDelta:0 };

  if (a.segments.length > b.segments.length) {
    return { relation:'left-inside-right', depthDelta:a.segments.length - b.segments.length };
  }
  if (b.segments.length > a.segments.length) {
    return { relation:'right-inside-left', depthDelta:b.segments.length - a.segments.length };
  }
  return { relation:'different', depthDelta:0 };
}

export function relativePaneLocation(location: string, root: string): string[] | null {
  const target = split(location), base = split(root);
  if (target.prefix !== base.prefix || target.segments.length < base.segments.length) return null;
  for (let i = 0; i < base.segments.length; i++) {
    if (!sameSegment(target.segments[i], base.segments[i])) return null;
  }
  return target.segments.slice(base.segments.length);
}

export function mirrorPaneLocation(sourceLocation: string, sourceRoot: string, targetRoot: string): string | null {
  const relative = relativePaneLocation(sourceLocation, sourceRoot);
  if (!relative) return null;
  const target = targetRoot.replace(/[\\/]+$/, '');
  if (!relative.length) return targetRoot;
  const separator = targetRoot.includes('\\') && !targetRoot.includes('://') ? '\\' : '/';
  return `${target}${separator}${relative.join(separator)}`;
}


export function mirroredPeerCandidates(
  sourceLocation: string,
  sourceRoot: string,
  targetRoot: string,
): string[] {
  const relative = relativePaneLocation(sourceLocation, sourceRoot);
  if (!relative) return [];

  const target = targetRoot.replace(/[\\/]+$/, '');
  const separator = targetRoot.includes('\\') && !targetRoot.includes('://') ? '\\' : '/';
  const candidates: string[] = [];

  for (let length = relative.length; length >= 0; length -= 1) {
    const suffix = relative.slice(0, length);
    candidates.push(
      suffix.length ? `${target}${separator}${suffix.join(separator)}` : targetRoot,
    );
  }

  return Array.from(new Set(candidates));
}
