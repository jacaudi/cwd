import { describe, it, expect } from 'vitest';
import { timeMaskForWindow, timeAxisFormatter } from './timeFormat';

describe('timeFormat', () => {
  it('returns HH:mm for 24h', () => {
    expect(timeMaskForWindow('24h')).toBe('HH:mm');
  });
  it('returns MM-DD HH:mm for 7d', () => {
    expect(timeMaskForWindow('7d')).toBe('MM-DD HH:mm');
  });
  it('returns MM-DD for 30d', () => {
    expect(timeMaskForWindow('30d')).toBe('MM-DD');
  });
  it('formatter formats a Date in the local zone per the mask', () => {
    const f = timeAxisFormatter('24h');
    const d = new Date(2026, 4, 5, 14, 7, 25); // local; month 4 = May
    expect(f(d)).toBe('14:07');
  });
});
