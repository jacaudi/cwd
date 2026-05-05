import type { HistoryWindow } from '../../api/history';

/** dayjs-style mask per window. G2 v2's axis.x.labelFormatter uses these. */
export function timeMaskForWindow(window: HistoryWindow): string {
  switch (window) {
    case '24h':
      return 'HH:mm';
    case '7d':
      return 'MM-DD HH:mm';
    case '30d':
      return 'MM-DD';
  }
}

/** Returns a labelFormatter callback compatible with @ant-design/charts v2 axis spec. */
export function timeAxisFormatter(window: HistoryWindow): (d: Date | string | number) => string {
  const mask = timeMaskForWindow(window);
  return (d) => {
    const date = d instanceof Date ? d : new Date(d);
    const yyyy = date.getFullYear();
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const dd = String(date.getDate()).padStart(2, '0');
    const hh = String(date.getHours()).padStart(2, '0');
    const mi = String(date.getMinutes()).padStart(2, '0');
    return mask
      .replace('YYYY', String(yyyy))
      .replace('MM', mm)
      .replace('DD', dd)
      .replace('HH', hh)
      .replace('mm', mi);
  };
}
