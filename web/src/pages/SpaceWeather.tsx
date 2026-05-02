import { Empty, Typography } from "antd";

export default function SpaceWeather() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Space Weather</Typography.Title>
      <Empty description="SWPC 3-day G/S/R forecast arrives in Phase 2." />
    </div>
  );
}
