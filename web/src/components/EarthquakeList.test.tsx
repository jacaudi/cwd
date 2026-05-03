import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EarthquakeList } from './EarthquakeList';
import type { Quake } from '../api/types';

function mk(over: Partial<Quake>): Quake {
  return {
    id: 'usqx',
    magnitude: 5.5,
    place: '100km W of Town',
    time: '2026-05-02T10:00:00Z',
    updatedAt: '2026-05-02T10:05:00Z',
    lat: 40.0,
    lon: -120.0,
    depthKm: 12.0,
    tsunami: false,
    url: 'https://earthquake.usgs.gov/usqx',
    ...over,
  };
}

describe('EarthquakeList', () => {
  it('returns null when payload is empty', () => {
    const { container } = render(<EarthquakeList quakes={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a row per quake with place + magnitude + depth', () => {
    render(<EarthquakeList quakes={[mk({}), mk({ id: 'usqy', magnitude: 7.0, place: 'Ocean' })]} />);
    expect(screen.getByText('100km W of Town')).toBeInTheDocument();
    expect(screen.getByText('Ocean')).toBeInTheDocument();
    expect(screen.getAllByText(/12 km/).length).toBeGreaterThan(0);
  });

  it('uses tier-based magnitude color via data-mag-tier', () => {
    render(<EarthquakeList quakes={[
      mk({ id: '1', magnitude: 4.5 }),
      mk({ id: '2', magnitude: 5.5 }),
      mk({ id: '3', magnitude: 6.5 }),
      mk({ id: '4', magnitude: 7.5 }),
    ]} />);
    const tags = screen.getAllByTestId('quake-mag-tag');
    expect(tags[0].getAttribute('data-mag-tier')).toBe('lt5');
    expect(tags[1].getAttribute('data-mag-tier')).toBe('5to6');
    expect(tags[2].getAttribute('data-mag-tier')).toBe('6to7');
    expect(tags[3].getAttribute('data-mag-tier')).toBe('ge7');
  });

  it('renders tsunami icon when flag is true', () => {
    render(<EarthquakeList quakes={[mk({ tsunami: true })]} />);
    expect(screen.getByLabelText('tsunami-flagged')).toBeInTheDocument();
  });

  it('renders PAGER alert tag when present', () => {
    render(<EarthquakeList quakes={[mk({ alert: 'yellow' })]} />);
    expect(screen.getByText(/PAGER: yellow/i)).toBeInTheDocument();
  });

  it('renders external link to USGS page', () => {
    render(<EarthquakeList quakes={[mk({ url: 'https://earthquake.usgs.gov/usqx' })]} />);
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('usqx');
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
  });
});
