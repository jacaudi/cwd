import { Typography } from 'antd';
import { HazardCategoryCard, type HazardCategory } from '../components/HazardCategoryCard';

const CATEGORIES: HazardCategory[] = [
  'severe-storms',
  'wildfire',
  'excessive-rainfall',
  'winter',
  'heat',
  'tropical',
  'flooding',
];

// Issue #13: NCEP-style layout. Categories render as 7 stacked full-width rows
// rather than the prior 3-up grid. Each card carries its own marginBottom so
// rows sit flush vertically with minimal chrome between them — matches the
// reference NCEP CWD page where the 'Hazards' section is a vertical list of
// labeled horizontal strips.
export default function Hazards() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3} style={{ marginTop: 0 }}>Hazards</Typography.Title>
      {CATEGORIES.map((c) => (
        <HazardCategoryCard key={c} category={c} />
      ))}
    </div>
  );
}
