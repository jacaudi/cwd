import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
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

  it('does NOT render a separate color-code chip (color is conveyed by the alert-level chip)', () => {
    render(<VolcanoList volcanoes={[mk({ color: 'ORANGE' })]} />);
    // The literal "ORANGE" (or any color-code label) should not appear as a Tag.
    expect(screen.queryByText('ORANGE')).toBeNull();
    expect(screen.queryByText('YELLOW')).toBeNull();
    expect(screen.queryByText('RED')).toBeNull();
    expect(screen.queryByText('GREEN')).toBeNull();
  });

  it('renders exactly one chip per volcano (the alert-level chip)', () => {
    render(<VolcanoList volcanoes={[mk({ alert: 'WATCH', color: 'ORANGE' })]} />);
    const tags = screen.getAllByTestId('volcano-alert-tag');
    expect(tags).toHaveLength(1);
    // Sanity: scope a query inside the row's Space to assert no second Tag sibling
    // by looking at the AntD .ant-tag class count within the alert tag's parent.
    const row = tags[0].closest('.ant-list-item');
    expect(row).not.toBeNull();
    if (row) {
      const allTags = within(row as HTMLElement).getAllByText(/.+/, { selector: '.ant-tag' });
      expect(allTags).toHaveLength(1);
    }
  });

  it('renders external link', () => {
    render(<VolcanoList volcanoes={[mk({ url: 'https://volcanoes.usgs.gov/hans-public/notice/example' })]} />);
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('hans-public/notice/example');
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
  });

  // Guards against an upstream introducing a new AlertLevel value we haven't
  // mapped — the alert chip should still render with antd's `default` color
  // (a defined fallback) rather than passing `undefined` to <Tag color>.
  it('falls back to antd default color when alert value is unknown', () => {
    const rogue = mk({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      alert: 'FUTURE_LEVEL' as any,
    });
    expect(() => render(<VolcanoList volcanoes={[rogue]} />)).not.toThrow();
    const alertTag = screen.getByTestId('volcano-alert-tag');
    expect(alertTag.className).toContain('ant-tag-default');
  });
});
