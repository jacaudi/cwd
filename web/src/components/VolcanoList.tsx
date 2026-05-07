import { List, Space, Tag, Typography } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { AlertLevel, Volcano } from '../api/types';

interface Props {
  volcanoes: Volcano[];
}

const ALERT_COLOR: Record<AlertLevel, string> = {
  NORMAL:   'green',
  ADVISORY: 'gold',
  WATCH:    'orange',
  WARNING:  'red',
};

function relative(iso: string): string {
  if (!iso) return '';
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function VolcanoList({ volcanoes }: Props) {
  if (volcanoes.length === 0) return null;
  return (
    <List
      header={<Typography.Title level={5} style={{ margin: 0 }}>Volcanoes at elevated alert</Typography.Title>}
      dataSource={volcanoes}
      renderItem={(v) => (
        <List.Item
          actions={v.url ? [
            <a key="link" href={v.url} target="_blank" rel="noopener noreferrer">
              <ExportOutlined /> USGS
            </a>,
          ] : []}
        >
          <List.Item.Meta
            title={v.name}
            description={
              <Space size="small" wrap>
                <Typography.Text type="secondary">{v.region}</Typography.Text>
                <Tag
                  color={ALERT_COLOR[v.alert] ?? 'default'}
                  data-alert={v.alert}
                  data-testid="volcano-alert-tag"
                >
                  {v.alert}
                </Tag>
                {v.updatedAt && (
                  <Typography.Text type="secondary">· {relative(v.updatedAt)}</Typography.Text>
                )}
              </Space>
            }
          />
        </List.Item>
      )}
    />
  );
}
