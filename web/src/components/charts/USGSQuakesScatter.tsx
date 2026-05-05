import { Scatter } from '@ant-design/charts';
import { Empty } from 'antd';
import { useIsDark } from '../../theme';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface USGSQuakesScatterProps {
  data: Array<{ at: string; mag: number; place: string; depthKm: number }>;
  window: HistoryWindow;
}

export function USGSQuakesScatter({ data, window }: USGSQuakesScatterProps) {
  const isDark = useIsDark();
  if (data.length === 0) {
    return <Empty description="No events in this window" />;
  }
  const points = data.map((d) => ({ ...d, at: new Date(d.at) }));
  return (
    <Scatter
      data={points}
      xField="at"
      yField="mag"
      sizeField="mag"
      shapeField="circle"
      height={180}
      style={{ fill: '#722ed1', fillOpacity: 0.6 }}
      scale={{ y: { domainMin: 4, domainMax: 8 }, size: { range: [4, 16] } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'magnitude' },
      }}
      theme={isDark ? 'academy' : 'classic'}
    />
  );
}
