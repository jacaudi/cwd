import { List, Space, Tag, Typography } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { Quake } from '../api/types';

interface Props {
  quakes: Quake[];
}

type MagTier = 'lt5' | '5to6' | '6to7' | 'ge7';

function magTier(mag: number): MagTier {
  if (mag >= 7.0) return 'ge7';
  if (mag >= 6.0) return '6to7';
  if (mag >= 5.0) return '5to6';
  return 'lt5';
}

const TIER_COLOR: Record<MagTier, string> = {
  lt5:  'blue',
  '5to6': 'orange',
  '6to7': 'red',
  ge7:  'magenta',
};

function relative(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function EarthquakeList({ quakes }: Props) {
  if (quakes.length === 0) return null;
  return (
    <List
      header={<Typography.Title level={5} style={{ margin: 0 }}>Significant earthquakes</Typography.Title>}
      dataSource={quakes}
      renderItem={(q) => {
        const tier = magTier(q.magnitude);
        return (
          <List.Item
            actions={q.url ? [
              <a key="link" href={q.url} target="_blank" rel="noopener noreferrer">
                <ExportOutlined /> USGS
              </a>,
            ] : []}
          >
            <List.Item.Meta
              avatar={
                <Tag
                  color={TIER_COLOR[tier]}
                  data-mag-tier={tier}
                  data-testid="quake-mag-tag"
                >
                  M{q.magnitude.toFixed(1)}
                </Tag>
              }
              title={q.place}
              description={
                <Space size="small" wrap>
                  <Typography.Text type="secondary">{Math.round(q.depthKm)} km depth</Typography.Text>
                  <Typography.Text type="secondary">· {relative(q.time)}</Typography.Text>
                  {q.tsunami && <span aria-label="tsunami-flagged">🌊</span>}
                  {q.alert && <Tag color="purple">PAGER: {q.alert}</Tag>}
                </Space>
              }
            />
          </List.Item>
        );
      }}
    />
  );
}
