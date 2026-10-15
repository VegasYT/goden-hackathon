// Package metricstest читает значения метрик в тестах.
package metricstest

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// Value возвращает сумму значений серий метрики name, у которых есть все перечисленные метки
// (пары "имя", "значение"). Для гистограммы это число наблюдений. Метрики или серии нет - ноль.
func Value(t testing.TB, gatherer prometheus.Gatherer, name string, labels ...string) float64 {
	t.Helper()
	require.Zero(t, len(labels)%2, "labels are name-value pairs")
	families, err := gatherer.Gather()
	require.NoError(t, err)

	var total float64
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	series:
		for _, metric := range family.GetMetric() {
			for i := 0; i < len(labels); i += 2 {
				found := false
				for _, label := range metric.GetLabel() {
					if label.GetName() == labels[i] && label.GetValue() == labels[i+1] {
						found = true
					}
				}
				if !found {
					continue series
				}
			}
			switch {
			case metric.GetCounter() != nil:
				total += metric.GetCounter().GetValue()
			case metric.GetGauge() != nil:
				total += metric.GetGauge().GetValue()
			case metric.GetHistogram() != nil:
				total += float64(metric.GetHistogram().GetSampleCount())
			}
		}
	}
	return total
}

// Series возвращает число серий метрики name с перечисленными метками.
func Series(t testing.TB, gatherer prometheus.Gatherer, name string, labels ...string) int {
	t.Helper()
	families, err := gatherer.Gather()
	require.NoError(t, err)
	count := 0
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	series:
		for _, metric := range family.GetMetric() {
			for i := 0; i < len(labels); i += 2 {
				found := false
				for _, label := range metric.GetLabel() {
					if label.GetName() == labels[i] && label.GetValue() == labels[i+1] {
						found = true
					}
				}
				if !found {
					continue series
				}
			}
			count++
		}
	}
	return count
}
