import { useEffect, useState } from 'react';
import { Tag, Tooltip } from 'antd';
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

  // flex-wrap so the row of tags reflows onto multiple lines instead of
  // overflowing the footer when there are many sources (post-Phase 3 the
  // image proxy adds ~20 image:* entries on top of the 5 data sources).
  // The Tag component already supplies its own margin; row-gap covers the
  // wrapped lines so they don't visually collide.
  return (
    <div
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        rowGap: 4,
        columnGap: 8,
        alignItems: 'center',
      }}
    >
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
            <Tag color={color} style={{ marginInlineEnd: 0 }}>{name}</Tag>
          </Tooltip>
        );
      })}
    </div>
  );
}
