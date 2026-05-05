import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { ConfigProvider, theme as antTheme } from 'antd';
import type { ReactNode } from 'react';
import { useIsDark } from './theme';

function wrapWith(algorithm: typeof antTheme.darkAlgorithm | typeof antTheme.defaultAlgorithm) {
  return ({ children }: { children: ReactNode }) => (
    <ConfigProvider theme={{ algorithm }}>{children}</ConfigProvider>
  );
}

describe('useIsDark', () => {
  it('returns true under darkAlgorithm', () => {
    const { result } = renderHook(() => useIsDark(), {
      wrapper: wrapWith(antTheme.darkAlgorithm),
    });
    expect(result.current).toBe(true);
  });

  it('returns false under defaultAlgorithm', () => {
    const { result } = renderHook(() => useIsDark(), {
      wrapper: wrapWith(antTheme.defaultAlgorithm),
    });
    expect(result.current).toBe(false);
  });
});
