import { useEffect, useRef, useState } from 'react';
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
import { NWSEventCountsStrip } from './charts/NWSEventCountsStrip';
import { SWPCScalesLine } from './charts/SWPCScalesLine';
import { SWPCAlertsBars } from './charts/SWPCAlertsBars';
import { USGSQuakesScatter } from './charts/USGSQuakesScatter';
import { VolcanoesTimeline } from './charts/VolcanoesTimeline';
import { useSnapshotStore } from '../store/snapshot';

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

const SSE_REFETCH_DEBOUNCE_MS = 250;

function headlineFor(resp: Resp): string {
  switch (resp.source) {
    case 'nws_alerts': {
      const max = resp.buckets.reduce((m, b) => (b.activeCount > m ? b.activeCount : m), 0);
      return `${max} active`;
    }
    case 'swpc_scales': {
      const last = resp.buckets[resp.buckets.length - 1];
      return last ? `G${last.gScale}` : 'G0';
    }
    case 'swpc_alerts': {
      const total = resp.buckets.reduce((s, b) => s + b.warning + b.watch + b.alert, 0);
      return `${total} in window`;
    }
    case 'usgs_quakes':
      return `${resp.events.length} M4+`;
    case 'usgs_volcanoes':
      return `${resp.changes.length} elevated`;
  }
}

export function HistoryChartRow({ source, window }: HistoryChartRowProps) {
  const [resp, setResp] = useState<Resp | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  // Per-source SSE timestamp from the snapshot store. When this advances we
  // re-fetch /api/history (debounced 250ms to coalesce same-tick storms).
  const sseFetchedAt = useSnapshotStore(
    (s) => s.snapshot?.sources?.[source]?.fetchedAt ?? null,
  );

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let cancelled = false;
    const fetcher: Record<HistorySource, (w: HistoryWindow) => Promise<Resp>> = {
      nws_alerts: (w) => fetchNWSAlertsHistory(w),
      swpc_scales: (w) => fetchSWPCScalesHistory(w),
      swpc_alerts: (w) => fetchSWPCAlertsHistory(w),
      usgs_quakes: (w) => fetchUSGSQuakesHistory(w),
      usgs_volcanoes: (w) => fetchUSGSVolcanoesHistory(w),
    };
    const run = () => {
      setLoading(true);
      setErr(null);
      fetcher[source](window)
        .then((r) => { if (!cancelled) setResp(r); })
        .catch((e: unknown) => { if (!cancelled) setErr(String(e)); })
        .finally(() => { if (!cancelled) setLoading(false); });
    };

    // Mount + (source,window) change → fetch immediately.
    // sseFetchedAt change → debounce-fetch.
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(run, SSE_REFETCH_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [source, window, sseFetchedAt]);

  const dataStartLag =
    resp && new Date(resp.dataStart).getTime() > new Date(resp.windowStart).getTime() + 60_000;

  let body: React.ReactNode;
  if (loading && !resp) {
    body = <Skeleton active paragraph={{ rows: 3 }} />;
  } else if (err) {
    body = <Typography.Text type="danger">Error: {err}</Typography.Text>;
  } else if (!resp) {
    body = null;
  } else {
    switch (resp.source) {
      case 'nws_alerts':
        body = (
          <>
            <NWSEventCountsStrip buckets={resp.buckets} />
            <NWSAlertsArea data={resp.buckets} window={window} />
          </>
        );
        break;
      case 'swpc_scales':
        body = <SWPCScalesLine data={resp.buckets} window={window} />;
        break;
      case 'swpc_alerts':
        body = <SWPCAlertsBars data={resp.buckets} window={window} />;
        break;
      case 'usgs_quakes':
        body = <USGSQuakesScatter data={resp.events} window={window} />;
        break;
      case 'usgs_volcanoes':
        body = <VolcanoesTimeline data={resp.changes} />;
        break;
    }
  }

  const extra = (
    <>
      {resp ? <Tag>{headlineFor(resp)}</Tag> : null}
      {dataStartLag && resp ? (
        <Tag>data starts here → {new Date(resp.dataStart).toUTCString()}</Tag>
      ) : null}
    </>
  );

  return (
    <Card
      title={TITLES[source]}
      extra={extra}
      style={{ marginBottom: 16, width: '100%' }}
      data-testid={`history-row-${source}`}
    >
      {body}
    </Card>
  );
}
