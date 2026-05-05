import { useEffect, useState } from 'react';
import { Card, Skeleton, Tag, Typography } from 'antd';
import {
  fetchNWSAlertsHistory,
  fetchSWPCAlertsHistory,
  fetchSWPCScalesHistory,
  fetchUSGSQuakesHistory,
  fetchUSGSVolcanoesHistory,
  type HistoryWindow,
  type NWSAlertsHistory,
  type SWPCAlertsHistory,
  type SWPCScalesHistory,
  type USGSQuakesHistory,
  type USGSVolcanoesHistory,
} from '../api/history';
import { NWSAlertsArea } from './charts/NWSAlertsArea';
import { SWPCScalesLine } from './charts/SWPCScalesLine';
import { SWPCAlertsBars } from './charts/SWPCAlertsBars';
import { USGSQuakesScatter } from './charts/USGSQuakesScatter';
import { VolcanoesTimeline } from './charts/VolcanoesTimeline';

export type HistorySource =
  | 'nws_alerts'
  | 'swpc_scales'
  | 'swpc_alerts'
  | 'usgs_quakes'
  | 'usgs_volcanoes';

const TITLES: Record<HistorySource, string> = {
  nws_alerts: 'NWS alerts — active count over time',
  swpc_scales: "SWPC scales — today's G / R / S",
  swpc_alerts: 'SWPC alerts — by severity',
  usgs_quakes: 'USGS quakes — magnitude scatter',
  usgs_volcanoes: 'USGS volcanoes — state-change events',
};

interface HistoryChartRowProps {
  source: HistorySource;
  window: HistoryWindow;
}

type Resp =
  | NWSAlertsHistory
  | SWPCScalesHistory
  | SWPCAlertsHistory
  | USGSQuakesHistory
  | USGSVolcanoesHistory;

export function HistoryChartRow({ source, window }: HistoryChartRowProps) {
  const [resp, setResp] = useState<Resp | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setErr(null);
    const fetcher: Record<HistorySource, (w: HistoryWindow) => Promise<Resp>> = {
      nws_alerts: (w) => fetchNWSAlertsHistory(w),
      swpc_scales: (w) => fetchSWPCScalesHistory(w),
      swpc_alerts: (w) => fetchSWPCAlertsHistory(w),
      usgs_quakes: (w) => fetchUSGSQuakesHistory(w),
      usgs_volcanoes: (w) => fetchUSGSVolcanoesHistory(w),
    };
    fetcher[source](window)
      .then((r) => { if (!cancelled) setResp(r); })
      .catch((e: unknown) => { if (!cancelled) setErr(String(e)); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [source, window]);

  const dataStartLag =
    resp && new Date(resp.dataStart).getTime() > new Date(resp.windowStart).getTime() + 60_000;

  let body: React.ReactNode;
  if (loading) {
    body = <Skeleton active paragraph={{ rows: 3 }} />;
  } else if (err) {
    body = <Typography.Text type="danger">Error: {err}</Typography.Text>;
  } else if (!resp) {
    body = null;
  } else {
    switch (resp.source) {
      case 'nws_alerts':
        body = <NWSAlertsArea data={resp.buckets} />;
        break;
      case 'swpc_scales':
        body = <SWPCScalesLine data={resp.buckets} />;
        break;
      case 'swpc_alerts':
        body = <SWPCAlertsBars data={resp.buckets} />;
        break;
      case 'usgs_quakes':
        body = <USGSQuakesScatter data={resp.events} />;
        break;
      case 'usgs_volcanoes':
        body = <VolcanoesTimeline data={resp.changes} />;
        break;
    }
  }

  return (
    <Card
      title={TITLES[source]}
      extra={dataStartLag && resp ? <Tag>data starts here → {new Date(resp.dataStart).toUTCString()}</Tag> : undefined}
      style={{ marginBottom: 16 }}
      data-testid={`history-row-${source}`}
    >
      {body}
    </Card>
  );
}
