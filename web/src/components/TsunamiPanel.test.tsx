import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TsunamiPanel } from './TsunamiPanel';
import type { Alert } from '../api/types';

const tsu = (wfo = 'NTW'): Alert => ({
  id: 'a',
  event: 'Tsunami Warning',
  awips: 'TSUWCA',
  headline: 'Tsunami Warning issued',
  severity: 'Extreme',
  sent: '2026-05-02T11:00:00Z',
  effective: '2026-05-02T11:00:00Z',
  expires: '2026-05-02T17:00:00Z',
  areas: ['Coastal Northern California'],
  category: 'Tsunami',
  wfo,
});

describe('TsunamiPanel', () => {
  it('returns null when no TSU alerts', () => {
    const { container } = render(<TsunamiPanel alerts={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders one item per TSU alert with NWS product link', () => {
    render(<TsunamiPanel alerts={[tsu('NTW')]} />);
    expect(screen.getByText(/Tsunami Warning issued/)).toBeInTheDocument();
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('product.php');
    expect(link.href).toContain('issuedby=NTW');
  });

  it('uses AWIPS prefix to filter, not event text', () => {
    const a: Alert = { ...tsu(), event: 'Some Renamed Event' };
    render(<TsunamiPanel alerts={[a]} />);
    expect(screen.getByText(/Tsunami Warning issued/)).toBeInTheDocument();
  });
});
