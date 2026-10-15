import { reasonLabel } from '@entities/delivery';
import { formatDate, formatDateTime, isSameDay } from '@shared/lib/datetime';

import type { Movement } from '../api/stock';

interface MovementDay {
	key: string;
	label: string;
	movements: Movement[];
}

const titles: Record<Movement['type'], string> = {
	received: 'Получение партии',
	consumed: 'Расход по заказу',
	written_off: 'Списание',
	adjusted: 'Исправление остатка',
	expiry_warning: 'Предупреждение о сроке',
	expired: 'Срок истёк',
	returned: 'Возврат по отмене заказа',
};

function dayKey(date: Date): string {
	return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
}

/** Движения по дням в порядке ответа: «Сегодня, 12.10», «11.10», для другого года — «11.10.2025». */
export function movementDays(movements: Movement[], now: Date): MovementDay[] {
	const days: MovementDay[] = [];

	for (const movement of movements) {
		const at = new Date(movement.occurred_at);
		const key = dayKey(at);
		const last = days.at(-1);

		if (last?.key === key) {
			last.movements.push(movement);
			continue;
		}

		const date = formatDate(at, now);

		days.push({
			key,
			label: isSameDay(at, now) ? `Сегодня, ${date}` : date,
			movements: [movement],
		});
	}

	return days;
}

export function movementTitle(movement: Movement): string {
	return titles[movement.type];
}

/** Изменение остатка по всем партиям движения со знаком. */
export function movementDelta(movement: Movement): number {
	return movement.changes.reduce((sum, change) => sum + change.delta, 0);
}

/** Партии движения по номерам поставок: «партия № 111», «партии № 111, № 113». */
export function movementBatches(movement: Movement): string {
	const numbers = [
		...new Set(movement.changes.map((change) => change.delivery_number)),
	].map((number) => `№ ${number}`);

	return `${numbers.length > 1 ? 'партии' : 'партия'} ${numbers.join(', ')}`;
}

/** Строка подробностей под движением; у расхода её нет. */
export function movementDetails(
	movement: Movement,
	now: Date
): string | undefined {
	switch (movement.type) {
		case 'written_off':
			return [
				movement.reason && reasonLabel(movement.reason),
				movement.comment && `«${movement.comment}»`,
			]
				.filter(Boolean)
				.join(' · ');

		case 'adjusted':
			return [movement.reason, movement.comment].filter(Boolean).join(' · ');

		case 'expiry_warning':
			return movement.expires_at
				? `Годна до ${formatDateTime(movement.expires_at, now)}: срок подходит к концу`
				: undefined;

		case 'expired':
			return 'Остаток просрочен и ждёт списания';

		case 'received':
			return movement.expires_at
				? `годна до ${formatDateTime(movement.expires_at, now)}`
				: undefined;

		case 'consumed':
			return undefined;
	}
}
