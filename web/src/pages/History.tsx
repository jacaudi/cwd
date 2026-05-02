import { Empty, Typography } from "antd";

export default function History() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>History</Typography.Title>
      <Empty description="Time-slider replay over the SQLite ring buffer arrives in Phase 4." />
    </div>
  );
}
