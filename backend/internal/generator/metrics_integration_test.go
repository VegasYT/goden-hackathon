//go:build integration

package generator_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/observability/metricstest"
)

func TestSimulatorMetrics(t *testing.T) {
	s := apitest.New(t)
	transport := &flakyTransport{}
	client, err := oas.NewClient(s.URL+api.Prefix, s.Security(), oas.WithClient(transport))
	require.NoError(t, err)
	sim := newSimulator(s, client)
	registry := prometheus.NewRegistry()
	registry.MustRegister(sim.Collectors()...)
	actions := func(action, outcome string) float64 {
		return metricstest.Value(t, registry, "goden_generator_actions_total", "action", action, "outcome", outcome)
	}

	// пустой каталог: действию нечего делать, оно не считается
	_, err = sim.Deliver(ctx, site(s))
	require.NoError(t, err)
	require.Zero(t, actions("delivery", "done"))

	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	_, err = sim.Deliver(ctx, site(s))
	require.NoError(t, err)
	placed, err := sim.Order(ctx, site(s))
	require.NoError(t, err)
	require.EqualValues(t, 1, actions("delivery", "done"))
	require.EqualValues(t, 1, actions("order", "done"))
	require.EqualValues(t, 1, metricstest.Value(t, registry, "goden_generator_orders_total", "status", string(placed.Status)))

	// API недоступен
	transport.down.Store(true)
	_, err = sim.Deliver(ctx, site(s))
	require.Error(t, err)
	_, err = sim.Order(ctx, site(s))
	require.Error(t, err)
	require.EqualValues(t, 1, actions("delivery", "error"))
	require.EqualValues(t, 1, actions("order", "error"))
	require.EqualValues(t, 1, actions("delivery", "done"))
}
