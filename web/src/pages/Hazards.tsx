import { Empty, Typography } from "antd";

export default function Hazards() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Hazards</Typography.Title>
      <Empty description="Severe storms, wildfire, rainfall, winter, heat, tropical, flooding maps arrive in Phase 3." />
    </div>
  );
}
