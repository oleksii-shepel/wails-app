import { describe, expect, it } from 'vitest';
import { buildActionAvailability } from './action-availability';
import type { SourceCapabilities } from './source-capabilities';
const rw: SourceCapabilities = { read:true, create:true, delete:true, rename:true, trash:true };
const ro: SourceCapabilities = { read:true, create:false, delete:false, rename:false, trash:false };
describe('buildActionAvailability', () => {
  it('enables full file actions for writable source and destination', () => {
    const a=buildActionAvailability({source:rw,destination:rw,selectedCount:1,hasClipboard:true,clipboardOperation:'copy'});
    expect(a.copyToOther).toBe(true); expect(a.moveToOther).toBe(true); expect(a.paste).toBe(true); expect(a.rename).toBe(true);
  });
  it('keeps copy-out but blocks move-out from read-only sources', () => {
    const a=buildActionAvailability({source:ro,destination:rw,selectedCount:1,hasClipboard:false});
    expect(a.copyToOther).toBe(true); expect(a.moveToOther).toBe(false); expect(a.deleteSelected).toBe(false);
  });
  it('blocks transfers into read-only destinations', () => {
    const a=buildActionAvailability({source:rw,destination:ro,selectedCount:1,hasClipboard:false});
    expect(a.copyToOther).toBe(false); expect(a.moveToOther).toBe(false);
  });
});
