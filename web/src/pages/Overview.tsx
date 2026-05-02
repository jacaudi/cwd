import { Empty, Typography } from "antd";

export default function Overview() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Overview</Typography.Title>
      <Empty description="Live status, alert badges, and 3-day outlook arrive in Phase 1." />
    </div>
  );
}
