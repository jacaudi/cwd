import { Empty, Typography } from "antd";

export default function Events() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Events</Typography.Title>
      <Empty description="Tsunamis, earthquakes, and volcano alerts arrive in Phase 2." />
    </div>
  );
}
