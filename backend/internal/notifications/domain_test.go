package notifications

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDomainMessageText(t *testing.T) {
	batchID := uuid.MustParse("0198c0de-0000-7000-8000-000000000002")
	expiresAt := time.Date(2026, 10, 12, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "warning",
			got:  expiryWarningText("Сливки", batchID, quantityText(400, "ml"), expiresAt),
			want: "⚠️ Срок годности скоро истечёт\nПродукт: Сливки\nПартия: 0198c0de-0000-7000-8000-000000000002\nОстаток: 400 мл\nГоден до: 12.10.2026 15:30 UTC",
		},
		{
			name: "expired",
			got:  batchExpiredText("Сливки", batchID, quantityText(250, "pcs"), expiresAt),
			want: "❌ Партия просрочена: использовать нельзя, нужно решение о списании\nПродукт: Сливки\nПартия: 0198c0de-0000-7000-8000-000000000002\nОстаток: 250 шт\nСрок годности: 12.10.2026 15:30 UTC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, tt.got) })
	}
}
