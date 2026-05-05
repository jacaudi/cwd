import { Area } from '@ant-design/charts';
import { Empty } from 'antd';

export interface NWSAlertsAreaProps {
  data: Array<{ at: string; activeCount: number }>;
}

export function NWSAlertsArea({ data }: NWSAlertsAreaProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  return (
    <Area
      data={data}
      xField="at"
      yField="activeCount"
      shapeField="smooth"
      height={180}
      style={{ fillOpacity: 0.15, fill: '#52c41a', stroke: '#52c41a' }}
      axis={{ x: { type: 'time' }, y: { domainMin: 0 } }}
    />
  );
}
