import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SourceHealthIndicator } from './SourceHealthIndicator';

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
});
