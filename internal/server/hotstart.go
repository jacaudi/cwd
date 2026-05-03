package server

import (
	"encoding/json"
	"fmt"

	"github.com/jacaudi/cwd/internal/sources"
)

// hotStartDecode decodes a stored payload row into the typed Go shape that
// matches the source's wire contract. Returns an error for unknown sources
// so server boot fails loud rather than silently storing the wrong type
// in the cache.
func hotStartDecode(name string, raw []byte) (any, error) {
	switch name {
	case sources.NWSAlertsName:
		var p []sources.Alert
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.SWPCScalesName:
		var p sources.SWPCForecast
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.SWPCAlertsName:
		var p []sources.SWPCAlert
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.USGSQuakesName:
		var p []sources.Quake
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.USGSVolcanoesName:
		var p []sources.Volcano
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("hot-start: unknown source %q", name)
	}
}
