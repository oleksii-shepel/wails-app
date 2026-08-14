import '@angular/compiler';
import { describe, expect, it } from 'vitest';
import {
  canonicalizeLocation,
  describeLocation,
  kindOfLocation,
  locationIdentity,
  locationLabel,
  schemeOfLocation,
} from './location-reference';

describe('location-reference', () => {
  it('classifies reusable search locations as queries', () => {
    const location = 'search://?path=filesystem://c/Work&pattern=invoice&recursive=true';
    expect(schemeOfLocation(location)).toBe('search');
    expect(kindOfLocation(location)).toBe('query');
    expect(describeLocation(location)).toMatchObject({ scheme: 'search', kind: 'query' });
    expect(locationLabel(location)).toBe('invoice');
  });

  it('preserves non-filesystem schemes as opaque canonical locations', () => {
    const location = 's3://archive/2026/reports';
    expect(canonicalizeLocation(location)).toBe(location);
    expect(locationIdentity(location)).toBe(`s3:${location}`);
  });

  it('uses case-insensitive identity for filesystem locations', () => {
    expect(locationIdentity('C:/Work')).toBe(locationIdentity('c:/work'));
  });
});
