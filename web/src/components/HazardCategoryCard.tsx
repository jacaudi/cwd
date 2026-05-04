import { useEffect, useMemo, useState } from 'react';
import { Image, Segmented, Skeleton, Tag } from 'antd';
import { ProCard } from '@ant-design/pro-components';
import { useSnapshotStore } from '../store/snapshot';

export type HazardCategory =
  | 'severe-storms'
  | 'wildfire'
  | 'excessive-rainfall'
  | 'winter'
  | 'heat'
  | 'tropical'
  | 'flooding';

interface ImageRef {
  key: string;
  label: string;
  path: string;
  alt: string;
}

const CATEGORIES: Record<HazardCategory, { title: string; images: ImageRef[] }> = {
  'severe-storms': { title: 'Severe Storms', images: [
    { key: 'spc.day1otlk', label: 'Day 1', path: '/img/spc/day1otlk', alt: 'SPC Convective Outlook Day 1' },
    { key: 'spc.day2otlk', label: 'Day 2', path: '/img/spc/day2otlk', alt: 'SPC Convective Outlook Day 2' },
    { key: 'spc.day3otlk', label: 'Day 3', path: '/img/spc/day3otlk', alt: 'SPC Convective Outlook Day 3' },
  ]},
  'wildfire': { title: 'Wildfire', images: [
    { key: 'spc.day1otlk_fire', label: 'Day 1', path: '/img/spc/day1otlk_fire', alt: 'SPC Fire Weather Outlook Day 1' },
    { key: 'spc.day2otlk_fire', label: 'Day 2', path: '/img/spc/day2otlk_fire', alt: 'SPC Fire Weather Outlook Day 2' },
    { key: 'spc.day38otlk_fire', label: 'Day 3-8', path: '/img/spc/day38otlk_fire', alt: 'SPC Fire Weather Outlook Day 3-8 (experimental)' },
  ]},
  'excessive-rainfall': { title: 'Excessive Rainfall', images: [
    { key: 'wpc.ero_day1', label: 'Day 1', path: '/img/wpc/ero_day1', alt: 'WPC Excessive Rainfall Outlook Day 1' },
    { key: 'wpc.ero_day2', label: 'Day 2', path: '/img/wpc/ero_day2', alt: 'WPC Excessive Rainfall Outlook Day 2' },
    { key: 'wpc.ero_day3', label: 'Day 3', path: '/img/wpc/ero_day3', alt: 'WPC Excessive Rainfall Outlook Day 3' },
  ]},
  'winter': { title: 'Winter', images: [
    { key: 'wpc.wssi_day1', label: 'Day 1', path: '/img/wpc/wssi_day1', alt: 'WPC Winter Storm Severity Index Day 1' },
    { key: 'wpc.wssi_day2', label: 'Day 2', path: '/img/wpc/wssi_day2', alt: 'WPC Winter Storm Severity Index Day 2' },
    { key: 'wpc.wssi_day3', label: 'Day 3', path: '/img/wpc/wssi_day3', alt: 'WPC Winter Storm Severity Index Day 3' },
  ]},
  'heat': { title: 'Heat', images: [
    { key: 'wpc.heatrisk_day1', label: 'Day 1', path: '/img/wpc/heatrisk_day1', alt: 'WPC HeatRisk Day 1' },
    { key: 'wpc.heatrisk_day2', label: 'Day 2', path: '/img/wpc/heatrisk_day2', alt: 'WPC HeatRisk Day 2' },
    { key: 'wpc.heatrisk_day3', label: 'Day 3', path: '/img/wpc/heatrisk_day3', alt: 'WPC HeatRisk Day 3' },
  ]},
  'tropical': { title: 'Tropical', images: [
    { key: 'nhc.atl_7d', label: 'Atlantic', path: '/img/nhc/atl_7d', alt: 'NHC Atlantic 7-day Outlook' },
    { key: 'nhc.epac_7d', label: 'East Pacific', path: '/img/nhc/epac_7d', alt: 'NHC East Pacific 7-day Outlook' },
    { key: 'nhc.cpac_7d', label: 'Central Pacific', path: '/img/nhc/cpac_7d', alt: 'NHC Central Pacific 7-day Outlook' },
    { key: 'navy.jtwc_abpw', label: 'JTWC ABPW', path: '/img/navy/jtwc_abpw', alt: 'Navy JTWC ABPW Western Pacific' },
  ]},
  'flooding': { title: 'Flooding', images: [
    { key: 'nwc.fho_national', label: 'National', path: '/img/nwc/fho_national', alt: 'WPC NWC National Flood Hazard Outlook' },
  ]},
};

interface ImageHealth {
  intervalSec: number;
  lastAttempt: string;
  lastSuccess: string;
  consecutiveFailures: number;
}

function StaleTag({ health }: { health: ImageHealth | undefined }) {
  if (!health || health.consecutiveFailures <= 0 || !health.lastSuccess) return null;
  const ageMs = Date.now() - new Date(health.lastSuccess).getTime();
  if (ageMs <= 0) return null;
  const minutes = Math.floor(ageMs / 60000);
  const label = minutes >= 1 ? `stale ${minutes}m` : `stale ${Math.floor(ageMs / 1000)}s`;
  return <Tag color="warning">{label}</Tag>;
}

export function HazardCategoryCard({ category }: { category: HazardCategory }) {
  const { title, images } = CATEGORIES[category];
  const [day, setDay] = useState(0);
  const imageRefresh = useSnapshotStore((s) => s.imageRefresh);
  const current = images[day];
  const refreshKey = imageRefresh[current.key] ?? 'init';

  const [healths, setHealths] = useState<Record<string, ImageHealth>>({});
  useEffect(() => {
    let aborted = false;
    const tick = async () => {
      try {
        const res = await fetch('/api/sources');
        if (!res.ok || aborted) return;
        const all = (await res.json()) as Record<string, ImageHealth | unknown>;
        const out: Record<string, ImageHealth> = {};
        for (const k of Object.keys(all)) {
          if (k.startsWith('image:')) {
            out[k.substring('image:'.length)] = all[k] as ImageHealth;
          }
        }
        if (!aborted) setHealths(out);
      } catch {
        /* swallow — Tag just won't render */
      }
    };
    void tick();
    const id = setInterval(tick, 60_000);
    return () => { aborted = true; clearInterval(id); };
  }, []);

  const segmentedOptions = useMemo(
    () => images.map((img, idx) => ({ label: img.label, value: idx })),
    [images],
  );

  return (
    <ProCard
      title={title}
      bordered
      extra={<StaleTag key={current.key} health={healths[current.key]} />}
    >
      {images.length > 1 && (
        <Segmented
          options={segmentedOptions}
          value={day}
          onChange={(v) => setDay(Number(v))}
          style={{ marginBottom: 12 }}
        />
      )}
      <Image.PreviewGroup>
        <Image
          key={refreshKey}
          src={current.path}
          alt={current.alt}
          data-refresh-key={refreshKey}
          placeholder={<Skeleton.Image style={{ width: '100%', height: 240 }} active />}
          style={{ width: '100%', height: 'auto' }}
        />
      </Image.PreviewGroup>
    </ProCard>
  );
}
