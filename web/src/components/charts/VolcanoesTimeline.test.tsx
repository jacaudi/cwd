import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { VolcanoesTimeline } from './VolcanoesTimeline';

describe('VolcanoesTimeline', () => {
  it('renders a timeline item per state change', () => {
    render(<VolcanoesTimeline data={[
      { at: '2026-05-03T10:00:00Z', volcano: 'AVO Great Sitkin', prior: 'YELLOW', current: 'ORANGE' },
      { at: '2026-05-03T12:00:00Z', volcano: 'HVO Kilauea', prior: '', current: 'WATCH' },
    ]} />);
    expect(screen.getByText(/AVO Great Sitkin/)).toBeTruthy();
    expect(screen.getByText(/HVO Kilauea/)).toBeTruthy();
    // prior → current rendered
    expect(screen.getByText(/YELLOW.*ORANGE/)).toBeTruthy();
    // appearance (prior empty) renders as "newly elevated"
    expect(screen.getByText(/newly elevated|→ WATCH/i)).toBeTruthy();
  });

  it('renders an Empty placeholder when no changes', () => {
    render(<VolcanoesTimeline data={[]} />);
    expect(screen.getByText(/no state changes/i)).toBeTruthy();
  });
});
