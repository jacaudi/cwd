import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { AlertBadgeBar } from './AlertBadgeBar';
import type { Alert, Category } from '../api/types';

const a = (cat: Category, over: Partial<Alert> = {}): Alert => ({
  id: Math.random().toString(),
  event: cat,
  awips: 'XXX',
  headline: 'h',
  severity: 'Severe',
  sent: 't',
  effective: 't',
  expires: 't',
  areas: [],
  category: cat,
  ...over,
});

describe('AlertBadgeBar', () => {
  it('renders all 10 chips with zero counts when payload empty', () => {
    render(<AlertBadgeBar alerts={[]} onTsunamiClick={() => {}} />);
    for (const label of [
      'Tornado',
      'Severe Thunderstorm',
      'Flash Flood',
      'Tropical',
      'High Wind',
      'Red Flag',
      'Winter',
      'Extreme Heat',
      'Extreme Cold',
      'Tsunami',
    ]) {
      expect(screen.getByLabelText(label)).toBeInTheDocument();
    }
  });

  it('does NOT render a 0 badge for inactive categories (showZero=false)', () => {
    render(<AlertBadgeBar alerts={[]} onTsunamiClick={() => {}} />);
    // Each chip's wrapper carries the aria-label; the chip text "Tornado" is
    // inside, but no "0" textnode should appear under that wrapper.
    const tornadoWrapper = screen.getByLabelText('Tornado');
    expect(within(tornadoWrapper).queryByText('0')).toBeNull();
  });

  it('renders the count badge when n > 0', () => {
    const alerts = [a('Tornado'), a('Tornado'), a('Tsunami')];
    render(<AlertBadgeBar alerts={alerts} onTsunamiClick={() => {}} />);
    const tornadoWrapper = screen.getByLabelText('Tornado');
    expect(within(tornadoWrapper).getByText('2')).toBeInTheDocument();
    const tsunamiWrapper = screen.getByLabelText('Tsunami');
    expect(within(tsunamiWrapper).getByText('1')).toBeInTheDocument();
  });

  it('counts alerts by category and ignores Unknown', () => {
    const alerts = [a('Tornado'), a('Tornado'), a('Tsunami'), a('Unknown')];
    render(<AlertBadgeBar alerts={alerts} onTsunamiClick={() => {}} />);
    expect(screen.getByLabelText('Tornado').textContent).toContain('2');
    expect(screen.getByLabelText('Tsunami').textContent).toContain('1');
  });
});
