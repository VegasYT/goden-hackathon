package auth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

func TestPrincipalFromClaims(t *testing.T) {
	tests := []struct {
		name      string
		claims    string
		wantActor eventstore.Actor
		wantUser  User
		has       []string
		hasNot    []string
		wantErr   bool
	}{
		{
			name:      "user",
			claims:    `{"sub":"u-1","email":"approver@goden.local","name":"Анна Приёмщик","preferred_username":"approver","resource_access":{"goden-api":{"roles":["deliveries.view","deliveries.approve"]}}}`,
			wantActor: eventstore.Actor{Type: eventstore.ActorUser, ID: "approver"},
			wantUser:  User{ID: "u-1", Email: "approver@goden.local", Name: "Анна Приёмщик"},
			has:       []string{"deliveries.view", "deliveries.approve"}, hasNot: []string{"inventory.edit"},
		},
		{
			name:      "service account",
			claims:    `{"sub":"s-1","preferred_username":"service-account-goden-generator","client_id":"goden-generator","resource_access":{"goden-api":{"roles":["orders.accept"]}}}`,
			wantActor: eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"},
			wantUser:  User{ID: "s-1"},
			has:       []string{"orders.accept"},
		},
		{
			name:      "roles of other clients are not permissions",
			claims:    `{"sub":"u-2","preferred_username":"cook","resource_access":{"account":{"roles":["inventory.edit"]}}}`,
			wantActor: eventstore.Actor{Type: eventstore.ActorUser, ID: "cook"},
			wantUser:  User{ID: "u-2"},
			hasNot:    []string{"inventory.edit"},
		},
		{
			name:      "no roles at all",
			claims:    `{"sub":"u-3","preferred_username":"guest"}`,
			wantActor: eventstore.Actor{Type: eventstore.ActorUser, ID: "guest"},
			wantUser:  User{ID: "u-3"},
			hasNot:    []string{"catalog.view"},
		},
		// владение и назначения ведутся по sub: без него клиента не с чем сопоставить
		{name: "no subject", claims: `{"preferred_username":"cook"}`, wantErr: true},
		{name: "nobody", claims: `{"resource_access":{"goden-api":{"roles":["catalog.view"]}}}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c claims
			require.NoError(t, json.Unmarshal([]byte(tt.claims), &c))

			principal, err := c.principal("goden-api")
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidToken)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantActor, principal.Actor)
			require.Equal(t, tt.wantUser, principal.User)
			for _, permission := range tt.has {
				require.True(t, principal.Has(permission), permission)
			}
			for _, permission := range tt.hasNot {
				require.False(t, principal.Has(permission), permission)
			}
		})
	}
}

func TestDisabledGrantsEverything(t *testing.T) {
	principal, err := Disabled().Verify(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, DevActor, principal.Actor)
	require.Equal(t, DevUser, principal.User)
	require.True(t, principal.Has("generator.manage"))
	require.True(t, principal.Has("anything"))
}
