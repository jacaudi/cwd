import { Area } from '@ant-design/charts';
import { Empty } from 'antd';
import { useIsDark } from '../../theme';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface NWSAlertsAreaProps {
  data: Array<{ at: string; activeCount: number }>;
  window: HistoryWindow;
}

export function NWSAlertsArea({ data, window }: NWSAlertsAreaProps) {
  const isDark = useIsDark();
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const points = data.map((d) => ({ at: new Date(d.at), activeCount: d.activeCount }));
  return (
    <Area
      data={points}
      xField="at"
      yField="activeCount"
      shapeField="smooth"
      height={180}
      style={{ fillOpacity: 0.15, fill: '#52c41a', stroke: '#52c41a' }}
      scale={{ x: { type: 'time' }, y: { domainMin: 0 } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'active alerts' },
      }}
      point={{ shapeField: 'point', sizeField: 3, style: { fill: '#52c41a' } }}
      theme={isDark ? 'academy' : 'classic'}
    />
  );
}
