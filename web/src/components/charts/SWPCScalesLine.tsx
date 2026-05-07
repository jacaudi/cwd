import { Line } from '@ant-design/charts';
import { Empty } from 'antd';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface SWPCScalesLineProps {
  data: Array<{ at: string; gScale: number; r1: number; s1: number }>;
  window: HistoryWindow;
}

interface Flat {
  at: Date;
  series: 'G-scale' | 'R1' | 'S1';
  value: number;
}

export function SWPCScalesLine({ data, window }: SWPCScalesLineProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => {
    const at = new Date(b.at);
    return [
      { at, series: 'G-scale', value: b.gScale },
      { at, series: 'R1', value: b.r1 },
      { at, series: 'S1', value: b.s1 },
    ];
  });
  return (
    <Line
      data={flat}
      xField="at"
      yField="value"
      colorField="series"
      shapeField="hv"
      height={180}
      scale={{ color: { range: ['#fa8c16', '#f5222d', '#1890ff'] } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'value' },
      }}
      point={{ shapeField: 'point', sizeField: 3 }}
    />
  );
}
