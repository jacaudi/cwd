import { Space, Typography } from 'antd';
import { EarthquakeList } from '../components/EarthquakeList';
import { TsunamiPanel } from '../components/TsunamiPanel';
import { VolcanoList } from '../components/VolcanoList';
import { useSnapshotStore } from '../store/snapshot';

export default function Events() {
  const snapshot = useSnapshotStore((s) => s.snapshot);
  const alerts = snapshot?.sources.nws_alerts?.payload ?? [];
  const quakes = snapshot?.sources.usgs_quakes?.payload ?? [];
  const volcanoes = snapshot?.sources.usgs_volcanoes?.payload ?? [];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', padding: 24 }}>
      <Typography.Title level={3} style={{ margin: 0 }}>Events</Typography.Title>
      <div id="tsunami">
        <TsunamiPanel alerts={alerts} />
      </div>
      <EarthquakeList quakes={quakes} />
      <VolcanoList volcanoes={volcanoes} />
    </Space>
  );
}
