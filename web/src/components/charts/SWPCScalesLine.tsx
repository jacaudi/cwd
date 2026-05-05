import { Line } from '@ant-design/charts';
import { Empty } from 'antd';

export interface SWPCScalesLineProps {
  data: Array<{ at: string; gScale: number; r1: number; s1: number }>;
}

interface Flat {
  at: string;
  series: 'G-scale' | 'R1' | 'S1';
  value: number;
}

export function SWPCScalesLine({ data }: SWPCScalesLineProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => [
    { at: b.at, series: 'G-scale', value: b.gScale },
    { at: b.at, series: 'R1', value: b.r1 },
    { at: b.at, series: 'S1', value: b.s1 },
  ]);
  return (
    <Line
      data={flat}
      xField="at"
      yField="value"
      seriesField="series"
      stepType="hv"
      height={180}
      color={['#fa8c16', '#f5222d', '#1890ff']}
      lineStyle={(d: { series: string }) =>
        d.series === 'G-scale' ? { lineWidth: 2 } : { lineDash: [4, 2] }
      }
      xAxis={{ type: 'time' }}
    />
  );
}
