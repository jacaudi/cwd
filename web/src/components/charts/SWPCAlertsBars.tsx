import { Column } from '@ant-design/charts';
import { Empty } from 'antd';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface SWPCAlertsBarsProps {
  data: Array<{ at: string; warning: number; watch: number; alert: number }>;
  window: HistoryWindow;
}

interface Flat {
  at: Date;
  severity: 'warning' | 'watch' | 'alert';
  count: number;
}

export function SWPCAlertsBars({ data, window }: SWPCAlertsBarsProps) {
  if (data.every((b) => b.warning + b.watch + b.alert === 0)) {
    return <Empty description="No alerts in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => {
    const at = new Date(b.at);
    return [
      { at, severity: 'warning', count: b.warning },
      { at, severity: 'watch', count: b.watch },
      { at, severity: 'alert', count: b.alert },
    ];
  });
  return (
    <Column
      data={flat}
      xField="at"
      yField="count"
      colorField="severity"
      stack
      height={180}
      scale={{ color: { range: ['#f5222d', '#fa8c16', '#1890ff'] } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'count' },
      }}
    />
  );
}
