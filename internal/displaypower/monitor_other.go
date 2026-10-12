//go:build !windows || server

package displaypower

import "log/slog"

// Monitor is a no-op in non-Windows and headless server builds.
type Monitor struct{}

func New(func(bool), *slog.Logger) *Monitor { return &Monitor{} }
func (*Monitor) Intercept(uintptr, uint32, uintptr, uintptr) (uintptr, bool) {
	return 0, false
}
func (*Monitor) Close() {}
