import { useEffect, useRef } from 'react';
import { Card, Space, Typography } from 'antd';
import { AlertBadgeBar } from '../components/AlertBadgeBar';
import { TsunamiPanel } from '../components/TsunamiPanel';
import { useSnapshotStore } from '../store/snapshot';
import { connect } from '../api/stream';

export default function Overview() {
  const snapshot = useSnapshotStore((s) => s.snapshot);
  const setSnapshot = useSnapshotStore((s) => s.setSnapshot);
  const applyUpdate = useSnapshotStore((s) => s.applyUpdate);
  const setConnection = useSnapshotStore((s) => s.setConnection);
  const tsuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancel: (() => void) | undefined;
    void connect({
      onSnapshot: (s) => {
        setSnapshot(s);
        setConnection('live');
      },
      onUpdate: (name, env) => applyUpdate(name, env),
      onError: () => setConnection('error'),
    }).then((c) => {
      cancel = c;
    });
    return () => cancel?.();
  }, [setSnapshot, applyUpdate, setConnection]);

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
          onTsunamiClick={() => tsuRef.current?.scrollIntoView({ behavior: 'smooth' })}
        />
      </Card>
      <div ref={tsuRef}>
        <TsunamiPanel alerts={alerts} />
      </div>
    </Space>
  );
}
