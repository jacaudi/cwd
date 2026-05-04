import { Col, Row, Typography } from 'antd';
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

export default function Hazards() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3} style={{ marginTop: 0 }}>Hazards</Typography.Title>
      <Row gutter={[16, 16]}>
        {CATEGORIES.map((c) => (
          <Col key={c} xs={24} sm={12} md={8}>
            <HazardCategoryCard category={c} />
          </Col>
        ))}
      </Row>
    </div>
  );
}
