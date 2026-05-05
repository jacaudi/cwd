import { describe, it, expect, beforeEach } from 'vitest';
import { useHistoryStore } from './history';

describe('useHistoryStore', () => {
  beforeEach(() => {
    useHistoryStore.setState({ window: '24h' });
  });

  it('defaults to 24h', () => {
    expect(useHistoryStore.getState().window).toBe('24h');
  });

  it('setWindow updates the value', () => {
    useHistoryStore.getState().setWindow('7d');
    expect(useHistoryStore.getState().window).toBe('7d');
  });

  it('parseWindowFromURL returns the param when valid', () => {
    expect(useHistoryStore.getState().parseWindowFromURL('?w=7d')).toBe('7d');
    expect(useHistoryStore.getState().parseWindowFromURL('?w=30d')).toBe('30d');
  });

  it('parseWindowFromURL returns null when missing or invalid', () => {
    expect(useHistoryStore.getState().parseWindowFromURL('')).toBeNull();
    expect(useHistoryStore.getState().parseWindowFromURL('?w=99d')).toBeNull();
    expect(useHistoryStore.getState().parseWindowFromURL('?other=x')).toBeNull();
  });
});
