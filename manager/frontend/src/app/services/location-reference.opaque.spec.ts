import '@angular/compiler';
import {
  canonicalizeLocation,
  locationIdentity,
  locationLabel,
  schemeOfLocation,
} from './location-reference';

describe('location-reference opaque plugin locations', () => {
  it('preserves provider-owned casing byte-for-byte', () => {
    const location = 'registry://HKEY_LOCAL_MACHINE/SYSTEM/Keyboard Layout';
    expect(canonicalizeLocation(location)).toBe(location);
    expect(locationIdentity(location)).toBe(`registry:${location}`);
    expect(locationLabel(location)).toBe('Keyboard Layout');
  });

  it('does not apply web-host semantics to the first plugin path component', () => {
    const location = 'registry://HKEY_LOCAL_MACHINE';
    expect(locationLabel(location)).toBe('HKEY_LOCAL_MACHINE');
  });

  it('keeps scheme matching case-insensitive without rewriting the location', () => {
    const location = 'Registry://HKEY_CURRENT_USER/Software';
    expect(schemeOfLocation(location)).toBe('registry');
    expect(canonicalizeLocation(location)).toBe(location);
  });

  it('still extracts query labels without URL host parsing', () => {
    expect(locationLabel('search://?pattern=Invoice&q=ignored')).toBe('Invoice');
  });
});
