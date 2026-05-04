import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { HazardCategoryCard } from './HazardCategoryCard';
import { useSnapshotStore } from '../store/snapshot';

afterEach(() => {
  useSnapshotStore.setState({ snapshot: null, connection: 'connecting', imageRefresh: {} });
  vi.restoreAllMocks();
});

beforeEach(() => {
  // /api/sources fetch returns no failures so no Tag renders by default.
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({}),
  }));
});

describe('HazardCategoryCard', () => {
  it('renders the title for the category', () => {
    render(<HazardCategoryCard category="severe-storms" />);
    expect(screen.getByText(/Severe Storms/i)).toBeInTheDocument();
  });

  it('renders the first day image by default', () => {
    render(<HazardCategoryCard category="severe-storms" />);
    const img = screen.getByAltText(/SPC Convective Outlook Day 1/i) as HTMLImageElement;
    expect(img.src).toContain('/img/spc/day1otlk');
  });

  it('switches images when the segmented day changes', async () => {
    render(<HazardCategoryCard category="severe-storms" />);
    fireEvent.click(screen.getByText('Day 2'));
    await waitFor(() => {
      const img = screen.getByAltText(/SPC Convective Outlook Day 2/i) as HTMLImageElement;
      expect(img.src).toContain('/img/spc/day2otlk');
    });
  });

  it('bumps the image element key when imageRefresh updates', async () => {
    const { rerender } = render(<HazardCategoryCard category="severe-storms" />);
    const beforeImg = screen.getByAltText(/SPC Convective Outlook Day 1/i);
    // antd's <Image> wraps the <img> in a div that receives data-* props; walk up.
    const beforeKey = beforeImg.closest('[data-refresh-key]')?.getAttribute('data-refresh-key');
    useSnapshotStore.getState().applyImageInvalidate({
      source: 'spc', name: 'day1otlk', fetchedAt: '2026-05-03T22:14:33Z',
    });
    rerender(<HazardCategoryCard category="severe-storms" />);
    const afterImg = screen.getByAltText(/SPC Convective Outlook Day 1/i);
    const afterKey = afterImg.closest('[data-refresh-key]')?.getAttribute('data-refresh-key');
    // React identity may stay the same; key bump triggers a remount.
    // Assert the key has been bumped via the data-refresh-key data attribute
    // the component sets for testability.
    expect(afterKey).toBe('2026-05-03T22:14:33Z');
    expect(beforeKey ?? 'init').not.toBe('2026-05-03T22:14:33Z');
  });

  it('shows a stale Tag when /api/sources reports consecutive failures', async () => {
    const past = new Date(Date.now() - 8 * 60 * 1000).toISOString();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        'image:spc.day1otlk': {
          intervalSec: 120,
          lastAttempt: new Date().toISOString(),
          lastSuccess: past,
          consecutiveFailures: 3,
        },
      }),
    }));
    render(<HazardCategoryCard category="severe-storms" />);
    await waitFor(() => {
      expect(screen.getByText(/stale 8m/i)).toBeInTheDocument();
    });
  });

  it('renders the flooding category with a single image', () => {
    render(<HazardCategoryCard category="flooding" />);
    expect(screen.getByAltText(/National Flood Hazard Outlook/i)).toBeInTheDocument();
  });
});
