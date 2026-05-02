import { Drawer, Form, Radio } from "antd";
import type { ThemeMode } from "../api/types";

interface Props {
  open: boolean;
  onClose: () => void;
  themeMode: ThemeMode;
  onThemeChange: (mode: ThemeMode) => void;
}

export default function SettingsDrawer({ open, onClose, themeMode, onThemeChange }: Props) {
  return (
    <Drawer title="Settings" open={open} onClose={onClose} placement="right" width={360}>
      <Form layout="vertical">
        <Form.Item label="Theme" tooltip="Server default applies on first load; your choice persists locally.">
          <Radio.Group
            value={themeMode}
            onChange={(e) => onThemeChange(e.target.value as ThemeMode)}
            optionType="button"
            buttonStyle="solid"
            options={[
              { label: "Dark",  value: "dark"  },
              { label: "Light", value: "light" },
              { label: "Auto",  value: "auto"  },
            ]}
          />
        </Form.Item>
        {/* Server-side knobs (intervals, thresholds, prewarm) are read-only via /api/uiconfig
            in v1 per design §6.2; richer Settings UI lands in Phase 6+. */}
      </Form>
    </Drawer>
  );
}
