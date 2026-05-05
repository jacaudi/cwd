import { Card, Space, Typography } from 'antd';
import { Tiny } from '@ant-design/charts';
import { useIsDark } from '../../theme';
import type { NWSAlertsHistory } from '../../api/history';

type Buckets = NWSAlertsHistory['buckets'];

interface Category {
  key: 'tornado' | 'severeTstorm' | 'flashFlood';
  label: string;
  color: string;
}

const CATEGORIES: Category[] = [
  { key: 'tornado',      label: 'Tornado',             color: '#f5222d' },
  { key: 'severeTstorm', label: 'Severe Thunderstorm', color: '#fa8c16' },
  { key: 'flashFlood',   label: 'Flash Flood',         color: '#1890ff' },
];

export interface NWSEventCountsStripProps {
  buckets: Buckets;
}

export function NWSEventCountsStrip({ buckets }: NWSEventCountsStripProps) {
  const isDark = useIsDark();
  const latest = buckets.length > 0 ? buckets[buckets.length - 1] : null;
  return (
    <Space size="small" wrap style={{ marginBottom: 12, width: '100%' }}>
      {CATEGORIES.map((c) => {
        const series = buckets.map((b) => b.eventCounts[c.key]);
        const latestCount = latest ? latest.eventCounts[c.key] : 0;
        return (
          <Card key={c.key} size="small" style={{ minWidth: 180, flex: '1 1 180px' }} styles={{ body: { padding: 12 } }}>
            <Space direction="vertical" size={2} style={{ width: '100%' }}>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>{c.label}</Typography.Text>
              <Typography.Text strong style={{ fontSize: 24, color: c.color }} data-testid={`count-${c.key}`}>
                {latestCount}
              </Typography.Text>
              {series.length > 0 ? (
                <Tiny.Area
                  data={series}
                  height={28}
                  width={140}
                  shapeField="smooth"
                  style={{ fill: c.color, fillOpacity: 0.2, stroke: c.color }}
                  theme={isDark ? 'academy' : 'classic'}
                />
              ) : (
                <div data-testid="sparkline" style={{ height: 28, width: 140 }} />
              )}
            </Space>
          </Card>
        );
      })}
    </Space>
  );
}
