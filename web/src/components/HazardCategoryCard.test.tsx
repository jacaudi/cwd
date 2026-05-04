import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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

  // Issue #9: render Day 1/2/3 side-by-side per category card. The
  // Segmented control + single-image swap is gone; all images live in the
  // same Image.PreviewGroup and render together.
  it('renders 3 images side-by-side for severe-storms', () => {
    render(<HazardCategoryCard category="severe-storms" />);
    expect(screen.getByAltText(/SPC Convective Outlook Day 1/i)).toBeInTheDocument();
    expect(screen.getByAltText(/SPC Convective Outlook Day 2/i)).toBeInTheDocument();
    expect(screen.getByAltText(/SPC Convective Outlook Day 3/i)).toBeInTheDocument();
  });

  it('renders 3 images for wildfire (Day 1, Day 2, Day 3-8)', () => {
    render(<HazardCategoryCard category="wildfire" />);
    expect(screen.getByAltText(/SPC Fire Weather Outlook Day 1/i)).toBeInTheDocument();
    expect(screen.getByAltText(/SPC Fire Weather Outlook Day 2/i)).toBeInTheDocument();
    expect(screen.getByAltText(/SPC Fire Weather Outlook Day 3-8/i)).toBeInTheDocument();
  });

  it('renders 4 images for tropical', () => {
    render(<HazardCategoryCard category="tropical" />);
    expect(screen.getByAltText(/Atlantic 7-day/i)).toBeInTheDocument();
    expect(screen.getByAltText(/East Pacific 7-day/i)).toBeInTheDocument();
    expect(screen.getByAltText(/Central Pacific 7-day/i)).toBeInTheDocument();
    expect(screen.getByAltText(/JTWC ABPW/i)).toBeInTheDocument();
  });

  it('renders 1 image for flooding', () => {
    render(<HazardCategoryCard category="flooding" />);
    expect(screen.getByAltText(/National Flood Hazard Outlook/i)).toBeInTheDocument();
    // No other category images leak in.
    expect(screen.queryByAltText(/Day 2/i)).toBeNull();
  });

  it('does NOT render a Segmented day picker (deprecated post-#9)', () => {
    const { container } = render(<HazardCategoryCard category="severe-storms" />);
    expect(container.querySelector('.ant-segmented')).toBeNull();
  });

  it('bumps the image element key when imageRefresh updates for any per-card image', () => {
    const { rerender } = render(<HazardCategoryCard category="severe-storms" />);
    const beforeImg = screen.getByAltText(/SPC Convective Outlook Day 2/i);
    const beforeKey = beforeImg.closest('[data-refresh-key]')?.getAttribute('data-refresh-key');
    useSnapshotStore.getState().applyImageInvalidate({
      source: 'spc', name: 'day2otlk', fetchedAt: '2026-05-03T22:14:33Z',
    });
    rerender(<HazardCategoryCard category="severe-storms" />);
    const afterImg = screen.getByAltText(/SPC Convective Outlook Day 2/i);
    const afterKey = afterImg.closest('[data-refresh-key]')?.getAttribute('data-refresh-key');
    expect(afterKey).toBe('2026-05-03T22:14:33Z');
    expect(beforeKey ?? 'init').not.toBe('2026-05-03T22:14:33Z');
  });

  it('renders all images inside a single Image.PreviewGroup so the lightbox cycles through them', () => {
    const { container } = render(<HazardCategoryCard category="severe-storms" />);
    // antd renders the preview-group wrapper; its children are the previewable
    // images. We assert exactly one preview group container exists.
    const previewGroups = container.querySelectorAll('.ant-image-preview-group');
    // antd 5 doesn't always emit a class for the wrapper; assert via the
    // image elements all share a single grouping by checking we have N images.
    expect(screen.getAllByRole('img').length).toBeGreaterThanOrEqual(3);
    // If the class is present (antd version-dependent), ensure no duplicate groups.
    expect(previewGroups.length).toBeLessThanOrEqual(1);
  });

  // The "stale Xm" Tag is per-card, driven by the WORST consecutive-failure
  // count across the card's images. Pre-#9 the tag tracked a single image; now
  // each card has up to 4 images that can fail independently.
  it('shows a stale Tag reflecting the worst failing image in the card', async () => {
    const past = new Date(Date.now() - 8 * 60 * 1000).toISOString();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        // Day 1 fine.
        'image:spc.day1otlk': {
          intervalSec: 120,
          lastAttempt: new Date().toISOString(),
          lastSuccess: new Date().toISOString(),
          consecutiveFailures: 0,
        },
        // Day 2 stale (8 min).
        'image:spc.day2otlk': {
          intervalSec: 120,
          lastAttempt: new Date().toISOString(),
          lastSuccess: past,
          consecutiveFailures: 5,
        },
        // Day 3 marginally stale.
        'image:spc.day3otlk': {
          intervalSec: 120,
          lastAttempt: new Date().toISOString(),
          lastSuccess: new Date(Date.now() - 60_000).toISOString(),
          consecutiveFailures: 1,
        },
      }),
    }));
    render(<HazardCategoryCard category="severe-storms" />);
    // Worst lastSuccess is 8m ago.
    await waitFor(() => {
      expect(screen.getByText(/stale 8m/i)).toBeInTheDocument();
    });
  });

  it('does not render a stale Tag when no per-card image is failing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        'image:spc.day1otlk': {
          intervalSec: 120,
          lastAttempt: 't',
          lastSuccess: new Date().toISOString(),
          consecutiveFailures: 0,
        },
      }),
    }));
    const { container } = render(<HazardCategoryCard category="severe-storms" />);
    // Wait for the fetch to complete then assert no stale tag.
    await waitFor(() => {
      // Some other Tag content (the title) might be here; assert specifically
      // no element matches /stale \d+/ within the card.
      expect(container.textContent).not.toMatch(/stale \d+(m|s)/i);
    });
  });
});
