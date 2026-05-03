import { Empty, List, Space, Tag, Typography } from 'antd';
import type { SWPCAlert } from '../api/types';

interface Props {
  alerts: SWPCAlert[];
}

const SERIES_COLOR: Record<string, string> = {
  'K-Index':            'magenta',
  'Geomagnetic-Watch':  'volcano',
  'Proton-Event':       'gold',
  'Radio-Blackout':     'red',
  'X-Ray-Flare':        'orange',
  'Radio-Sweep':        'cyan',
  'Discussion':         'default',
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

function truncate(s: string, max = 120): string {
  if (s.length <= max) return s;
  return s.slice(0, max - 1) + '…';
}

export function SWPCAlerts({ alerts }: Props) {
  if (alerts.length === 0) {
    return <Empty description="No active SWPC alerts in window" />;
  }
  return (
    <List
      dataSource={alerts}
      renderItem={(a) => (
        <List.Item>
          <List.Item.Meta
            avatar={
              <Tag
                color={SERIES_COLOR[a.series] ?? 'default'}
                data-series={a.series}
              >
                {a.code}
              </Tag>
            }
            title={
              <Space size="small">
                <Typography.Text strong>{a.series}</Typography.Text>
                <Typography.Text type="secondary">{a.description}</Typography.Text>
                <Typography.Text type="secondary">· {relative(a.issued)}</Typography.Text>
              </Space>
            }
            description={truncate(a.message)}
          />
        </List.Item>
      )}
    />
  );
}
