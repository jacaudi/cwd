import { Scatter } from '@ant-design/charts';
import { Empty } from 'antd';

export interface USGSQuakesScatterProps {
  data: Array<{ at: string; mag: number; place: string; depthKm: number }>;
}

export function USGSQuakesScatter({ data }: USGSQuakesScatterProps) {
  if (data.length === 0) {
    return <Empty description="No events in this window" />;
  }
  return (
    <Scatter
      data={data}
      xField="at"
      yField="mag"
      sizeField="mag"
      size={[4, 16]}
      shape="circle"
      height={180}
      color="#722ed1"
      xAxis={{ type: 'time' }}
      yAxis={{ min: 4, max: 8 }}
      tooltip={{
        formatter: (d: { place?: string; mag?: number; depthKm?: number; at?: string }) => ({
          name: d.place ?? 'unknown',
          value: `M${d.mag?.toFixed?.(1) ?? '?'} · ${d.depthKm?.toFixed?.(0) ?? '?'} km`,
        }),
      }}
    />
  );
}
