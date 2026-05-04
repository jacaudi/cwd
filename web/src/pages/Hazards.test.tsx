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
});
