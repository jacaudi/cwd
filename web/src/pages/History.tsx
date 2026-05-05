import { useEffect } from 'react';
import { Segmented, Typography } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { HistoryChartRow, type HistorySource } from '../components/HistoryChartRow';
import { useHistoryStore } from '../store/history';
import type { HistoryWindow } from '../api/history';

const SOURCES: HistorySource[] = [
  'nws_alerts',
  'swpc_scales',
  'swpc_alerts',
  'usgs_quakes',
  'usgs_volcanoes',
];

export default function History() {
  const window = useHistoryStore((s) => s.window);
  const setWindow = useHistoryStore((s) => s.setWindow);
  const parseWindow = useHistoryStore((s) => s.parseWindowFromURL);
  const location = useLocation();
  const navigate = useNavigate();

  // On mount + on URL change, seed window from ?w= if present.
  useEffect(() => {
    const fromURL = parseWindow(location.search);
    if (fromURL && fromURL !== window) {
      setWindow(fromURL);
    }
    // intentionally not including `window` in deps — URL is the input,
    // store is the output.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.search, parseWindow, setWindow]);

  const onWindowChange = (w: HistoryWindow) => {
    setWindow(w);
    const params = new URLSearchParams(location.search);
    params.set('w', w);
    navigate({ pathname: location.pathname, search: '?' + params.toString() }, { replace: true });
  };

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 16 }}>
        <Typography.Title level={3} style={{ marginTop: 0, marginBottom: 0 }}>
          History
        </Typography.Title>
        <Segmented
          options={['24h', '7d', '30d']}
          value={window}
          onChange={(v) => onWindowChange(v as HistoryWindow)}
        />
      </div>
      {SOURCES.map((s) => (
        <HistoryChartRow key={s} source={s} window={window} />
      ))}
    </div>
  );
}
