import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { VolcanoList } from './VolcanoList';
import type { Volcano } from '../api/types';

function mk(over: Partial<Volcano>): Volcano {
  return {
    id: '311100',
    name: 'Redoubt',
    region: 'Alaska Volcano Observatory',
    alert: 'WATCH',
    color: 'ORANGE',
    updatedAt: '2026-05-02T10:00:00Z',
    url: 'https://volcanoes.usgs.gov/hans-public/notice/example',
    ...over,
  };
}

describe('VolcanoList', () => {
  it('returns null when payload is empty', () => {
    const { container } = render(<VolcanoList volcanoes={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a row per volcano with name + region', () => {
    render(<VolcanoList volcanoes={[mk({}), mk({ id: 'b', name: 'Shasta', region: 'California Volcano Observatory' })]} />);
    expect(screen.getByText('Redoubt')).toBeInTheDocument();
    expect(screen.getByText('Shasta')).toBeInTheDocument();
    expect(screen.getByText(/Alaska Volcano Observatory/)).toBeInTheDocument();
    expect(screen.getByText(/California Volcano Observatory/)).toBeInTheDocument();
  });

  it('renders alert-level tag color via data-alert', () => {
    render(<VolcanoList volcanoes={[
      mk({ id: 'a', alert: 'ADVISORY' }),
      mk({ id: 'b', name: 'B', alert: 'WATCH' }),
      mk({ id: 'c', name: 'C', alert: 'WARNING' }),
    ]} />);
    const tags = screen.getAllByTestId('volcano-alert-tag');
    expect(tags[0].getAttribute('data-alert')).toBe('ADVISORY');
    expect(tags[1].getAttribute('data-alert')).toBe('WATCH');
    expect(tags[2].getAttribute('data-alert')).toBe('WARNING');
  });

  it('renders the color-code chip', () => {
    render(<VolcanoList volcanoes={[mk({ color: 'ORANGE' })]} />);
    expect(screen.getByText('ORANGE')).toBeInTheDocument();
  });

  it('renders external link', () => {
    render(<VolcanoList volcanoes={[mk({ url: 'https://volcanoes.usgs.gov/hans-public/notice/example' })]} />);
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('hans-public/notice/example');
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
  });
});
