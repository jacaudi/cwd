import { useEffect, useState } from 'react';
import { Col, Image, Row, Skeleton, Tag } from 'antd';
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

// Per-card column width in AntD's 24-col grid. Mobile (xs) collapses to
// full-width regardless. Desktop (sm+) gets 3-up for 3-image cards, 4-up for
// the 4-image Tropical card, and full-width for the 1-image Flooding card.
function colSpanFor(count: number): { xs: number; sm: number } {
  if (count >= 4) return { xs: 24, sm: 6 };
  if (count >= 2) return { xs: 24, sm: 8 };
  return { xs: 24, sm: 24 };
}

// WorstStaleTag computes the maximum staleness across all per-card images and
// renders a single tag based on the worst case. Returns null when no image in
// the card is currently failing — operators only see noise when something is
// actually wrong.
function WorstStaleTag({ images, healths }: { images: ImageRef[]; healths: Record<string, ImageHealth> }) {
  let worst: ImageHealth | undefined;
  let worstAge = -1;
  for (const img of images) {
    const h = healths[img.key];
    if (!h || h.consecutiveFailures <= 0 || !h.lastSuccess) continue;
    const ageMs = Date.now() - new Date(h.lastSuccess).getTime();
    if (ageMs <= 0) continue;
    if (ageMs > worstAge) {
      worst = h;
      worstAge = ageMs;
    }
  }
  if (!worst || worstAge <= 0) return null;
  const minutes = Math.floor(worstAge / 60000);
  const label = minutes >= 1 ? `stale ${minutes}m` : `stale ${Math.floor(worstAge / 1000)}s`;
  return <Tag color="warning">{label}</Tag>;
}

export function HazardCategoryCard({ category }: { category: HazardCategory }) {
  const { title, images } = CATEGORIES[category];
  const imageRefresh = useSnapshotStore((s) => s.imageRefresh);
  const span = colSpanFor(images.length);

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

  return (
    <ProCard
      title={title}
      bordered
      extra={<WorstStaleTag images={images} healths={healths} />}
    >
      {/*
        Issue #9: render Day 1/2/3 (and Tropical's 4 basins, and Flooding's
        single national map) side-by-side in a single Image.PreviewGroup. The
        lightbox cycles through all images in the group via arrow keys, so the
        Segmented day-picker is no longer needed. Each <Image> is keyed on the
        SSE-driven imageRefresh entry for that key — Phase 3's invalidation
        path is unchanged.
      */}
      <Image.PreviewGroup>
        <Row gutter={[8, 8]}>
          {images.map((img) => {
            const refreshKey = imageRefresh[img.key] ?? 'init';
            return (
              <Col key={img.key} xs={span.xs} sm={span.sm}>
                <Image
                  key={refreshKey}
                  src={img.path}
                  alt={img.alt}
                  data-refresh-key={refreshKey}
                  placeholder={<Skeleton.Image style={{ width: '100%', height: 240 }} active />}
                  style={{ width: '100%', height: 'auto' }}
                />
              </Col>
            );
          })}
        </Row>
      </Image.PreviewGroup>
    </ProCard>
  );
}
