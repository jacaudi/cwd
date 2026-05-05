import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SourceHealthIndicator } from './SourceHealthIndicator';
import { CONTENT_MAX_WIDTH } from '../layout/constants';

describe('SourceHealthIndicator', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders one tag per source, color reflects health', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        nws_alerts: {
          intervalSec: 30,
          lastAttempt: 't',
          lastSuccess: 't',
          ageSec: 5,
          consecutiveFailures: 0,
        },
      }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    render(<SourceHealthIndicator pollMs={50} />);
    await waitFor(() => expect(screen.getByText(/nws_alerts/)).toBeInTheDocument());
  });

  // Issue #7: with 25 source tags (5 data sources + 20 image:* entries) the
  // footer must wrap onto multiple lines, not overflow horizontally. The
  // container is a flexbox with flex-wrap: wrap.
  it('wraps tags onto multiple lines instead of overflowing', async () => {
    const many: Record<string, unknown> = {};
    for (let i = 0; i < 25; i++) {
      many[`image:src${i}.name`] = {
        intervalSec: 60,
        lastAttempt: 't',
        lastSuccess: 't',
        ageSec: 1,
        consecutiveFailures: 0,
      };
    }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => many,
    }));
    const { container } = render(<SourceHealthIndicator pollMs={50} />);
    await waitFor(() => expect(screen.getByText('image:src0.name')).toBeInTheDocument());
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper).toBeTruthy();
    expect(wrapper.style.display).toBe('flex');
    expect(wrapper.style.flexWrap).toBe('wrap');
  });

  // Issue #12: the footer's outer container must share the page content
  // max-width so the wrapped tag wall doesn't stretch past the right edge of
  // the cards above it on wide viewports. The single source of truth lives
  // in web/src/layout/constants.ts (CONTENT_MAX_WIDTH); App.tsx applies the
  // same value to the main content wrapper.
  it('constrains the outer container to the shared content max-width', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        nws_alerts: {
          intervalSec: 30,
          lastAttempt: 't',
          lastSuccess: 't',
          ageSec: 5,
          consecutiveFailures: 0,
        },
      }),
    }));
    const { container } = render(<SourceHealthIndicator pollMs={50} />);
    await waitFor(() => expect(screen.getByText('nws_alerts')).toBeInTheDocument());
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper).toBeTruthy();
    expect(wrapper.style.maxWidth).toBe(`${CONTENT_MAX_WIDTH}px`);
    // Centered horizontally so the constrained container aligns with the
    // page content above it.
    expect(wrapper.style.marginLeft).toBe('auto');
    expect(wrapper.style.marginRight).toBe('auto');
    // Fill available width up to the cap.
    expect(wrapper.style.width).toBe('100%');
  });
});
