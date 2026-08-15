import { beforeEach, describe, expect, it } from 'vitest';
import { LEGACY_PINNED_PATHS_KEY, PINNED_LOCATIONS_KEY } from '../components/address-bar.types';
import { PinnedLocationService } from './pinned-location.service';

describe('PinnedLocationService', () => {
  let service: PinnedLocationService;

  beforeEach(() => {
    localStorage.clear();
    service = new PinnedLocationService();
  });

  it('classifies searches as query locations', () => {
    const pin = service.create('search://?path=/src&pattern=TODO&contentSearch=true');
    expect(pin?.scheme).toBe('search');
    expect(pin?.kind).toBe('query');
    expect(pin?.location).toContain('pattern=TODO');
  });

  it('classifies ordinary paths as filesystem locations', () => {
    const pin = service.create('/home/user/projects');
    expect(pin?.scheme).toBe('filesystem');
    expect(pin?.kind).toBe('location');
  });

  it('migrates legacy pinned paths', () => {
    localStorage.setItem(LEGACY_PINNED_PATHS_KEY, JSON.stringify([{
      path: '/home/user/projects',
      label: 'Projects',
      icon: '📁',
      addedAt: 123,
    }]));

    const pins = service.load();
    expect(pins).toHaveLength(1);
    expect(pins[0].label).toBe('Projects');
    expect(pins[0].location).toContain('/home/user/projects');
    expect(localStorage.getItem(PINNED_LOCATIONS_KEY)).toBeTruthy();
  });

  it('uses canonical identity for filesystem pins', () => {
    const pin = service.create('/home/user/projects');
    expect(pin).not.toBeNull();
    expect(service.isPinned([pin!], '/home/user/projects')).toBe(true);
  });

  it('persists semantic surface identity without storing runtime instances', () => {
    const pin = service.create('search://?path=/src&pattern=TODO', undefined, undefined, 123, 'search-results');
    expect(pin?.surfaceId).toBe('search-results');
    expect(pin?.id).toContain('|surface:search-results');

    service.save([pin!]);
    const [restored] = service.load();
    expect(restored.surfaceId).toBe('search-results');
    expect(restored.location).toBe(pin?.location);
  });

  it('allows the same durable location to be pinned for different surfaces', () => {
    const location = 'search://?path=/src&pattern=TODO';
    const dirPin = service.create(location, undefined, undefined, 1, 'dir')!;
    const resultsPin = service.create(location, undefined, undefined, 2, 'search-results')!;

    expect(dirPin.id).not.toBe(resultsPin.id);
    expect(service.isPinned([dirPin, resultsPin], location, 'dir')).toBe(true);
    expect(service.isPinned([dirPin, resultsPin], location, 'search-results')).toBe(true);
    expect(service.remove([dirPin, resultsPin], location, 'dir')).toEqual([resultsPin]);
  });

  it('infers surface identity from surface:// locations', () => {
    const pin = service.create('surface://console/session/main');
    expect(pin?.kind).toBe('surface');
    expect(pin?.surfaceId).toBe('console');
  });


  it('uses the dedicated search-results surface for search pins', () => {
    const pin = service.create('search://?path=/src&pattern=TODO');
    expect(pin?.surfaceId).toBe('search-results');
  });


  it('migrates search pins that were previously stored as dir surfaces', () => {
    localStorage.setItem(PINNED_LOCATIONS_KEY, JSON.stringify([{
      id: 'legacy-search', location: 'search://?path=/src&pattern=TODO', scheme: 'search', kind: 'query',
      surfaceId: 'dir', label: 'TODO', icon: '🔎', addedAt: 1,
    }]));
    const [pin] = service.load();
    expect(pin.surfaceId).toBe('search-results');
  });

});
