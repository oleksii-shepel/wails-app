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
});
