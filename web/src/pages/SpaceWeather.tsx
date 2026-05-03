import { Card, Space, Typography } from 'antd';
import { SWPCAlerts } from '../components/SWPCAlerts';
import { SWPCForecast } from '../components/SWPCForecast';
import { useSnapshotStore } from '../store/snapshot';

export default function SpaceWeather() {
  const snapshot = useSnapshotStore((s) => s.snapshot);
  const scales = snapshot?.sources.swpc_scales;
  const alerts = snapshot?.sources.swpc_alerts;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', padding: 24 }}>
      <Typography.Title level={3} style={{ margin: 0 }}>Space Weather</Typography.Title>
      <Card title="3-day NOAA scales forecast" extra={scales?.fetchedAt ? (
        <Typography.Text type="secondary">as of {new Date(scales.fetchedAt).toUTCString()}</Typography.Text>
      ) : null}>
        <SWPCForecast forecast={scales?.payload ?? null} fetchedAt={scales?.fetchedAt ?? null} />
      </Card>
      <Card title="Active SWPC alerts" extra={alerts?.fetchedAt ? (
        <Typography.Text type="secondary">as of {new Date(alerts.fetchedAt).toUTCString()}</Typography.Text>
      ) : null}>
        <SWPCAlerts alerts={alerts?.payload ?? []} />
      </Card>
    </Space>
  );
}
