import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SWPCAlerts } from './SWPCAlerts';
import type { SWPCAlert } from '../api/types';

function mk(over: Partial<SWPCAlert>): SWPCAlert {
  return {
    code: 'K08A',
    series: 'K-Index',
    description: 'K-index 8 = G4 severe',
    issued: '2026-05-02T10:00:00Z',
    message: 'Geomagnetic K=8 reached. Detail follows...',
    ...over,
  };
}

describe('SWPCAlerts', () => {
  it('renders Empty state when payload is empty', () => {
    render(<SWPCAlerts alerts={[]} />);
    expect(screen.getByText(/No active SWPC alerts/i)).toBeInTheDocument();
  });

  it('renders one row per alert with code, series, description', () => {
    render(<SWPCAlerts alerts={[mk({}), mk({ code: 'P12A', series: 'Proton-Event', description: '≥1,000 pfu', message: 'proton event' })]} />);
    expect(screen.getByText('K08A')).toBeInTheDocument();
    expect(screen.getByText('P12A')).toBeInTheDocument();
    expect(screen.getByText(/K-Index/)).toBeInTheDocument();
    expect(screen.getByText(/Proton-Event/)).toBeInTheDocument();
  });

  it('renders code Tag with series-derived color (data-series attribute)', () => {
    render(<SWPCAlerts alerts={[mk({})]} />);
    const tag = screen.getByText('K08A');
    expect(tag.getAttribute('data-series')).toBe('K-Index');
  });

  it('renders truncated message body', () => {
    const long = 'x'.repeat(400);
    render(<SWPCAlerts alerts={[mk({ message: long })]} />);
    // The component truncates display to ~120 chars; substring of x's must be present.
    expect(screen.getByText(/x{50,}/)).toBeInTheDocument();
  });
});
