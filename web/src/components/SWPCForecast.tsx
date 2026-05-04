import { Col, Row, Skeleton, Space, Statistic, Tag, Tooltip, Typography } from 'antd';
import { ProCard } from '@ant-design/pro-components';
import type { GScale, SWPCDay, SWPCForecast as SWPCForecastT } from '../api/types';

interface Props {
  forecast: SWPCForecastT | null;
  fetchedAt: string | null;
}

const DAY_LABELS = ['Tomorrow', '+2 days', '+3 days'];

const G_COLOR: Record<GScale, string> = {
  G1: 'green',
  G2: 'gold',
  G3: 'orange',
  G4: 'red',
  G5: 'magenta',
};

function severityColor(pct: number): string {
  if (pct >= 75) return '#ff4d4f';
  if (pct >= 50) return '#fa8c16';
  if (pct >= 25) return '#faad14';
  return '#52c41a';
}

function ProbRow({ label, value }: { label: string; value: number }) {
  return (
    <div style={{ marginBottom: 4 }}>
      <Space size="small">
        <Typography.Text strong>{label}</Typography.Text>
        <Statistic
          value={value}
          suffix="%"
          valueStyle={{ fontSize: 14, color: severityColor(value) }}
        />
      </Space>
    </div>
  );
}

function GChip({ day }: { day: SWPCDay }) {
  if (!day.g) {
    return (
      <Typography.Text type="secondary">G-scale: none</Typography.Text>
    );
  }
  return <Tag color={G_COLOR[day.g]}>{day.g}</Tag>;
}

export function SWPCForecast({ forecast, fetchedAt }: Props) {
  // Layout: explicit AntD Row+Col grid, 3-up at sm+ and full-width on xs.
  // Issue #8 — ProCard.Group direction="row" was rendering 1-per-row in this
  // nested-inside-<Card> context; matching the Hazards page's Row/Col idiom
  // gives explicit responsive control.
  if (!forecast) {
    return (
      <Row gutter={[16, 16]}>
        {DAY_LABELS.map((label) => (
          <Col key={label} xs={24} sm={8}>
            <ProCard title={label} bordered>
              <div role="status" aria-label="forecast-skeleton">
                <Skeleton active paragraph={{ rows: 3 }} />
              </div>
            </ProCard>
          </Col>
        ))}
      </Row>
    );
  }
  const fetchedNote = fetchedAt
    ? `as of ${new Date(fetchedAt).toUTCString()}`
    : '';
  return (
    <Row gutter={[16, 16]}>
      {DAY_LABELS.map((label, i) => {
        const day = forecast.days[i];
        const tooltipTitle = `${day.gText ?? ''} ${fetchedNote}`.trim();
        return (
          <Col key={label} xs={24} sm={8}>
            <Tooltip title={tooltipTitle || undefined}>
              <ProCard
                title={
                  <Space direction="vertical" size={0}>
                    <span>{label}</span>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {day.date}
                    </Typography.Text>
                  </Space>
                }
                bordered
                extra={<GChip day={day} />}
              >
                <ProbRow label="R1" value={day.r1} />
                <ProbRow label="R3" value={day.r3} />
                <ProbRow label="S1" value={day.s1} />
              </ProCard>
            </Tooltip>
          </Col>
        );
      })}
    </Row>
  );
}
