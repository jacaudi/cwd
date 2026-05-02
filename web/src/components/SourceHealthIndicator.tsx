import { useEffect, useState } from 'react';
import { Space, Tag, Tooltip } from 'antd';
import type { SourceHealth } from '../api/types';

interface Props {
  pollMs?: number;
}

export function SourceHealthIndicator({ pollMs = 15000 }: Props) {
  const [data, setData] = useState<Record<string, SourceHealth>>({});

  useEffect(() => {
    let alive = true;
    const tick = async () => {
      try {
        const r = await fetch('/api/sources');
        if (!r.ok) return;
        const json = await r.json();
        if (alive) setData(json);
      } catch {
        // swallow; tag stays last-known
      }
    };
    void tick();
    const id = setInterval(() => { void tick(); }, pollMs);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [pollMs]);

  return (
    <Space size="small">
      {Object.entries(data).map(([name, h]) => {
        const color =
          h.consecutiveFailures > 0
            ? 'red'
            : h.ageSec > 2 * h.intervalSec
              ? 'gold'
              : 'green';
        const tip = `${h.lastSuccess || 'never'} (age: ${h.ageSec}s, failures: ${h.consecutiveFailures})`;
        return (
          <Tooltip key={name} title={tip}>
            <Tag color={color}>{name}</Tag>
          </Tooltip>
        );
      })}
    </Space>
  );
}
