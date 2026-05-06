import { useMemo } from 'react';
import { Badge, Space, Tag, Tooltip } from 'antd';
import type { Alert, Category } from '../api/types';

const ORDER: { key: Exclude<Category, 'Unknown'>; label: string }[] = [
  { key: 'Tornado', label: 'Tornado' },
  { key: 'SevereThunderstorm', label: 'Severe Thunderstorm' },
  { key: 'FlashFlood', label: 'Flash Flood' },
  { key: 'Tropical', label: 'Tropical' },
  { key: 'HighWind', label: 'High Wind' },
  { key: 'RedFlag', label: 'Red Flag' },
  { key: 'Winter', label: 'Winter' },
  { key: 'ExtremeHeat', label: 'Extreme Heat' },
  { key: 'ExtremeCold', label: 'Extreme Cold' },
  { key: 'Tsunami', label: 'Tsunami' },
];

const COLOR: Record<Exclude<Category, 'Unknown'>, string> = {
  Tornado: 'red',
  SevereThunderstorm: 'volcano',
  FlashFlood: 'cyan',
  Tropical: 'magenta',
  HighWind: 'gold',
  RedFlag: 'orange',
  Winter: 'blue',
  ExtremeHeat: 'red',
  ExtremeCold: 'geekblue',
  Tsunami: 'purple',
};

interface Props {
  alerts: Alert[];
  onTsunamiClick: () => void;
}

export function AlertBadgeBar({ alerts, onTsunamiClick }: Props) {
  const counts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of alerts) m[a.category] = (m[a.category] ?? 0) + 1;
    return m;
  }, [alerts]);

  return (
    <Space size="small" wrap aria-label="active-alerts">
      {ORDER.map(({ key, label }) => {
        const n = counts[key] ?? 0;
        const color = n > 0 ? COLOR[key] : 'default';
        const onClick = key === 'Tsunami' ? onTsunamiClick : undefined;
        return (
          <Tooltip key={key} title={label}>
            <span aria-label={label}>
              <Badge count={n} showZero={false}>
                <Tag
                  color={color}
                  onClick={onClick}
                  style={{ cursor: onClick && n > 0 ? 'pointer' : 'default' }}
                >
                  {label}
                </Tag>
              </Badge>
            </span>
          </Tooltip>
        );
      })}
    </Space>
  );
}
