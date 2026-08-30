import { formatAttributeFlags } from './attribute-flags';

describe('attribute flag presentation', () => {
  it('renders native flags positionally like a permission mask', () => {
    expect(formatAttributeFlags('RS')).toBe('R-S-');
    expect(formatAttributeFlags('RHSA')).toBe('RHSA');
    expect(formatAttributeFlags('A')).toBe('---A');
    expect(formatAttributeFlags('rh')).toBe('RH--');
  });

  it('ignores unknown characters and non-string values', () => {
    expect(formatAttributeFlags('RX')).toBe('R---');
    expect(formatAttributeFlags(undefined)).toBe('');
    expect(formatAttributeFlags(42)).toBe('');
  });

  it('keeps Unix providers (no attribute characters) empty', () => {
    expect(formatAttributeFlags('')).toBe('');
    expect(formatAttributeFlags('   ')).toBe('');
  });
});
