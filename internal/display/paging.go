package display

import "time"

// DefaultCompactPageInterval preserves the 3.5-inch paging cadence used before it became configurable.
const DefaultCompactPageInterval = 10 * time.Second

// CompactPagingSettings controls automatic paging on the physical compact display.
// Window preview paging is intentionally independent.
type CompactPagingSettings struct {
	Auto     bool
	Interval time.Duration
}

func DefaultCompactPagingSettings() CompactPagingSettings {
	return CompactPagingSettings{Auto: true, Interval: DefaultCompactPageInterval}
}

func (p CompactPagingSettings) normalized() CompactPagingSettings {
	if p.Interval <= 0 {
		p.Interval = DefaultCompactPageInterval
	}
	return p
}
