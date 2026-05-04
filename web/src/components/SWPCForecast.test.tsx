import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SWPCForecast } from './SWPCForecast';
import type { SWPCForecast as SWPCForecastT } from '../api/types';

const fc: SWPCForecastT = {
  days: [
    { date: '2026-05-03', r1: 10, r3: 1,  s1: 1, g: 'G2', gText: 'G2 expected' },
    { date: '2026-05-04', r1: 30, r3: 5,  s1: 5, g: 'G4', gText: 'G4 watch' },
    { date: '2026-05-05', r1: 60, r3: 25, s1: 80, g: 'G5', gText: 'G5 extreme' },
  ],
};

describe('SWPCForecast', () => {
  it('renders three day cards with date subtitles', () => {
    render(<SWPCForecast forecast={fc} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getByText(/Tomorrow/)).toBeInTheDocument();
    expect(screen.getByText(/\+2 days/)).toBeInTheDocument();
    expect(screen.getByText(/\+3 days/)).toBeInTheDocument();
    expect(screen.getByText('2026-05-03')).toBeInTheDocument();
    expect(screen.getByText('2026-05-05')).toBeInTheDocument();
  });

  it('renders R1/R3/S1 percentages', () => {
    render(<SWPCForecast forecast={fc} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getAllByText(/R1/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/R3/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/S1/i).length).toBeGreaterThan(0);
  });

  it('renders G-scale chips with color', () => {
    render(<SWPCForecast forecast={fc} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getByText('G2')).toBeInTheDocument();
    expect(screen.getByText('G4')).toBeInTheDocument();
    expect(screen.getByText('G5')).toBeInTheDocument();
  });

  it('renders a muted "G-scale: none" label when g is absent', () => {
    const empty: SWPCForecastT = {
      days: [
        { date: '2026-05-03', r1: 5, r3: 0, s1: 0 },
        { date: '2026-05-04', r1: 5, r3: 0, s1: 0 },
        { date: '2026-05-05', r1: 5, r3: 0, s1: 0 },
      ],
    };
    render(<SWPCForecast forecast={empty} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getAllByText(/G-scale: none/i).length).toBe(3);
  });

  it('renders skeleton state when forecast is null', () => {
    render(<SWPCForecast forecast={null} fetchedAt={null} />);
    expect(screen.getAllByLabelText(/forecast-skeleton/i).length).toBe(3);
  });

  // Issue #8: forecast cards must lay out 3-up via AntD Row/Col, not stacked
  // in a vertical ProCard.Group. The wrapper exposes the .ant-row class so
  // we can assert the layout shape without coupling to specific style values.
  it('uses an AntD Row container with three Col children', () => {
    const { container } = render(<SWPCForecast forecast={fc} fetchedAt={null} />);
    const row = container.querySelector('.ant-row');
    expect(row).toBeTruthy();
    const cols = container.querySelectorAll('.ant-col');
    expect(cols.length).toBe(3);
  });

  it('uses Row/Col layout in the skeleton state too', () => {
    const { container } = render(<SWPCForecast forecast={null} fetchedAt={null} />);
    expect(container.querySelector('.ant-row')).toBeTruthy();
    expect(container.querySelectorAll('.ant-col').length).toBe(3);
  });
});
