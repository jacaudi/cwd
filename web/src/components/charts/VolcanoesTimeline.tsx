import { Empty, Timeline, Typography } from 'antd';

export interface VolcanoesTimelineProps {
  data: Array<{ at: string; volcano: string; prior: string; current: string }>;
}

function colorFor(level: string): string {
  switch (level.toUpperCase()) {
    case 'WARNING':
      return '#f5222d';
    case 'WATCH':
      return '#fa8c16';
    case 'ADVISORY':
      return '#fadb14';
    default:
      return '#8c8c8c';
  }
}

export function VolcanoesTimeline({ data }: VolcanoesTimelineProps) {
  if (data.length === 0) {
    return <Empty description="No state changes in this window" />;
  }
  return (
    <Timeline
      items={data.map((c) => ({
        color: colorFor(c.current || c.prior),
        children: (
          <>
            <Typography.Text type="secondary" style={{ fontSize: 11, marginRight: 8 }}>
              {new Date(c.at).toLocaleString(undefined, { timeZone: 'UTC' })} UTC
            </Typography.Text>
            <Typography.Text strong>{c.volcano}</Typography.Text>
            <Typography.Text style={{ marginLeft: 8 }}>
              {c.prior === ''
                ? `newly elevated → ${c.current}`
                : c.current === ''
                  ? `${c.prior} → removed`
                  : `${c.prior} → ${c.current}`}
            </Typography.Text>
          </>
        ),
      }))}
    />
  );
}
