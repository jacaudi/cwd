import { Column } from '@ant-design/charts';
import { Empty } from 'antd';

export interface SWPCAlertsBarsProps {
  data: Array<{ at: string; warning: number; watch: number; alert: number }>;
}

interface Flat {
  at: string;
  severity: 'warning' | 'watch' | 'alert';
  count: number;
}

export function SWPCAlertsBars({ data }: SWPCAlertsBarsProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => [
    { at: b.at, severity: 'warning', count: b.warning },
    { at: b.at, severity: 'watch', count: b.watch },
    { at: b.at, severity: 'alert', count: b.alert },
  ]);
  return (
    <Column
      data={flat}
      xField="at"
      yField="count"
      seriesField="severity"
      isStack
      height={180}
      color={['#f5222d', '#fa8c16', '#1890ff']}
      xAxis={{ type: 'time' }}
    />
  );
}
