import { useEffect, useState, type ReactNode } from 'react';
import { Card, Col, Image, Row, Skeleton, Space, Tag, Typography } from 'antd';
import {
  CloudOutlined,
  ExperimentOutlined,
  FireOutlined,
  GlobalOutlined,
  SunOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
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

const CATEGORIES: Record<HazardCategory, { title: string; icon: ReactNode; images: ImageRef[] }> = {
  'severe-storms': { title: 'Severe Storms', icon: <ThunderboltOutlined />, images: [
    { key: 'spc.day1otlk', label: 'Day 1', path: '/img/spc/day1otlk', alt: 'SPC Convective Outlook Day 1' },
    { key: 'spc.day2otlk', label: 'Day 2', path: '/img/spc/day2otlk', alt: 'SPC Convective Outlook Day 2' },
    { key: 'spc.day3otlk', label: 'Day 3', path: '/img/spc/day3otlk', alt: 'SPC Convective Outlook Day 3' },
  ]},
  'wildfire': { title: 'Wildfire', icon: <FireOutlined />, images: [
    { key: 'spc.day1otlk_fire', label: 'Day 1', path: '/img/spc/day1otlk_fire', alt: 'SPC Fire Weather Outlook Day 1' },
    { key: 'spc.day2otlk_fire', label: 'Day 2', path: '/img/spc/day2otlk_fire', alt: 'SPC Fire Weather Outlook Day 2' },
    { key: 'spc.day38otlk_fire', label: 'Day 3-8', path: '/img/spc/day38otlk_fire', alt: 'SPC Fire Weather Outlook Day 3-8 (experimental)' },
  ]},
  'excessive-rainfall': { title: 'Excessive Rainfall', icon: <CloudOutlined />, images: [
    { key: 'wpc.ero_day1', label: 'Day 1', path: '/img/wpc/ero_day1', alt: 'WPC Excessive Rainfall Outlook Day 1' },
    { key: 'wpc.ero_day2', label: 'Day 2', path: '/img/wpc/ero_day2', alt: 'WPC Excessive Rainfall Outlook Day 2' },
    { key: 'wpc.ero_day3', label: 'Day 3', path: '/img/wpc/ero_day3', alt: 'WPC Excessive Rainfall Outlook Day 3' },
  ]},
  'winter': { title: 'Winter', icon: <ExperimentOutlined />, images: [
    { key: 'wpc.wssi_day1', label: 'Day 1', path: '/img/wpc/wssi_day1', alt: 'WPC Winter Storm Severity Index Day 1' },
    { key: 'wpc.wssi_day2', label: 'Day 2', path: '/img/wpc/wssi_day2', alt: 'WPC Winter Storm Severity Index Day 2' },
    { key: 'wpc.wssi_day3', label: 'Day 3', path: '/img/wpc/wssi_day3', alt: 'WPC Winter Storm Severity Index Day 3' },
  ]},
  'heat': { title: 'Heat', icon: <SunOutlined />, images: [
    { key: 'wpc.heatrisk_day1', label: 'Day 1', path: '/img/wpc/heatrisk_day1', alt: 'WPC HeatRisk Day 1' },
    { key: 'wpc.heatrisk_day2', label: 'Day 2', path: '/img/wpc/heatrisk_day2', alt: 'WPC HeatRisk Day 2' },
    { key: 'wpc.heatrisk_day3', label: 'Day 3', path: '/img/wpc/heatrisk_day3', alt: 'WPC HeatRisk Day 3' },
  ]},
  'tropical': { title: 'Tropical', icon: <GlobalOutlined />, images: [
    { key: 'nhc.atl_7d', label: 'Atlantic', path: '/img/nhc/atl_7d', alt: 'NHC Atlantic 7-day Outlook' },
    { key: 'nhc.epac_7d', label: 'East Pacific', path: '/img/nhc/epac_7d', alt: 'NHC East Pacific 7-day Outlook' },
    { key: 'nhc.cpac_7d', label: 'Central Pacific', path: '/img/nhc/cpac_7d', alt: 'NHC Central Pacific 7-day Outlook' },
    { key: 'navy.jtwc_abpw', label: 'JTWC ABPW', path: '/img/navy/jtwc_abpw', alt: 'Navy JTWC ABPW Western Pacific' },
  ]},
  'flooding': { title: 'Flooding', icon: <CloudOutlined />, images: [
    { key: 'nwc.fho_national', label: 'National', path: '/img/nwc/fho_national', alt: 'WPC NWC National Flood Hazard Outlook' },
  ]},
};

interface ImageHealth {
  intervalSec: number;
  lastAttempt: string;
  lastSuccess: string;
  consecutiveFailures: number;
}

// Per-image column width inside the right-hand image strip. The strip itself
// already uses ~20/24 of the page row, so these spans are computed against the
// strip's inner 24-col grid: 3 maps → 8 each, 4 maps (Tropical) → 6 each, 1 map
// (Flooding) → full width. Mobile (xs) collapses to full width.
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
  const { title, icon, images } = CATEGORIES[category];
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

  /*
    Issue #13: NCEP-style page-wide row layout. Each category renders as a
    horizontal row with a narrow left header cell (icon+title+stale tag stacked)
    and a right image strip cell that takes the remaining width. On mobile (xs)
    the header collapses ABOVE the strip; on desktop (md+) it sits to the left.

    No ProCard chrome — categories sit flush vertically; the page provides the
    inter-row spacing via marginBottom on the outer wrapper.

    Issue #9 contract preserved: all images live inside a single
    Image.PreviewGroup so the lightbox cycles through them, and each <Image>
    is keyed on the SSE-driven imageRefresh entry for that key (Phase 3
    invalidation path is unchanged).
  */
  return (
    <Card
      data-testid="hazard-row"
      style={{ marginBottom: 24 }}
      styles={{ body: { padding: 16 } }}
    >
      <Row align="top" gutter={[16, 12]} wrap>
        <Col
          xs={24}
          md={4}
          data-testid="hazard-row-header"
          style={{ paddingTop: 4 }}
        >
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Space size={8} align="center">
              <span style={{ fontSize: 18, lineHeight: 1 }}>{icon}</span>
              <Typography.Text strong style={{ fontSize: 16 }}>{title}</Typography.Text>
            </Space>
            <WorstStaleTag images={images} healths={healths} />
          </Space>
        </Col>
        <Col xs={24} md={20}>
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
        </Col>
      </Row>
    </Card>
  );
}
