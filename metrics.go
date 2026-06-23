package config

import "time"

type MetricsSink interface {
	ObserveConfigLoad(time.Duration, bool)
	IncReload(bool)
	IncRuntimeMutation(bool)
	SetVersion(uint64)
}
type NoopMetricsSink struct{}

func (NoopMetricsSink) ObserveConfigLoad(time.Duration, bool) {}
func (NoopMetricsSink) IncReload(bool)                        {}
func (NoopMetricsSink) IncRuntimeMutation(bool)               {}
func (NoopMetricsSink) SetVersion(uint64)                     {}
