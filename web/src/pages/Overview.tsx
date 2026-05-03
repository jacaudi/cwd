import { Card, Space, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import { AlertBadgeBar } from '../components/AlertBadgeBar';
import { useSnapshotStore } from '../store/snapshot';

export default function Overview() {
  const navigate = useNavigate();
  const snapshot = useSnapshotStore((s) => s.snapshot);

  // The SSE stream is opened once at the App level (see App.tsx). Overview
  // (and every other route) reads from the shared snapshot store, so deep-
  // links to other pages still get live data without each page wiring its
  // own EventSource.

  const alerts = snapshot?.sources.nws_alerts?.payload ?? [];
  const fetchedAt = snapshot?.sources.nws_alerts?.fetchedAt;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', padding: 24 }}>
      <Typography.Title level={3} style={{ margin: 0 }}>Overview</Typography.Title>
      <Card
        title="Active Alerts"
        extra={fetchedAt ? (
          <Typography.Text type="secondary">
            as of {new Date(fetchedAt).toUTCString()}
          </Typography.Text>
        ) : null}
      >
        <AlertBadgeBar
          alerts={alerts}
          onTsunamiClick={() => navigate('/events#tsunami')}
        />
      </Card>
    </Space>
  );
}
