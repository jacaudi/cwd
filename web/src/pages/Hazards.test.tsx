import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import Hazards from './Hazards';

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }));
});

describe('Hazards page', () => {
  it('renders all 7 category cards', () => {
    render(<Hazards />);
    for (const title of ['Severe Storms', 'Wildfire', 'Excessive Rainfall', 'Winter', 'Heat', 'Tropical', 'Flooding']) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
  });

  it('renders the page heading', () => {
    render(<Hazards />);
    expect(screen.getByRole('heading', { name: /Hazards/i })).toBeInTheDocument();
  });

  // Issue #13: NCEP layout — categories stack vertically as page-wide rows,
  // not the prior 3-up grid. Each row exposes data-testid="hazard-row".
  it('renders 7 categories as stacked rows (no 3-up grid)', () => {
    const { container } = render(<Hazards />);
    const rows = container.querySelectorAll('[data-testid="hazard-row"]');
    expect(rows.length).toBe(7);
  });
});
