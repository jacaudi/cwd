import { Avatar, List, Tag } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { Alert } from '../api/types';

interface Props {
  alerts: Alert[];
}

export function TsunamiPanel({ alerts }: Props) {
  const tsu = alerts.filter((a) => a.awips.startsWith('TSU'));
  if (tsu.length === 0) return null;
  return (
    <List
      header={<strong>Tsunami</strong>}
      dataSource={tsu}
      renderItem={(a) => (
        <List.Item
          actions={[
            <a
              key="link"
              href={`https://forecast.weather.gov/product.php?site=NWS&product=TSU&issuedby=${a.wfo ?? ''}`}
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExportOutlined /> NWS product
            </a>,
          ]}
        >
          <List.Item.Meta
            avatar={<Avatar style={{ backgroundColor: '#722ed1' }}>T</Avatar>}
            title={a.headline}
            description={
              <span>
                {a.areas.join(', ')} <Tag>{new Date(a.sent).toUTCString()}</Tag>
              </span>
            }
          />
        </List.Item>
      )}
    />
  );
}
