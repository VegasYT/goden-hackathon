import { addDays } from 'date-fns';

import {
	dayMs,
	formatLongDateTime,
	formatTime,
	hourMs,
	isSameDay,
	minuteMs,
} from '@shared/lib/datetime';
import { expiryStatus } from '@shared/lib/expiry';
import { countOf } from '@shared/lib/plural';
import { formatAmount, type QuantityUnit } from '@shared/lib/quantity';

import type { StopListForecast } from '../api/analytics';

/** Целое число с разрядами: "14 195". */
export function formatCount(value: number): string {
	return value.toLocaleString('ru-RU');
}

/** Длительность: "меньше минуты", "40 мин", "4 ч 10 мин", "1 день 7 ч". */
export function formatDuration(ms: number): string {
	if (ms < minuteMs) {
		return 'меньше минуты';
	}

	const minutes = Math.floor((ms % hourMs) / minuteMs);

	if (ms < hourMs) {
		return `${minutes} мин`;
	}

	const hours = Math.floor((ms % dayMs) / hourMs);

	if (ms < dayMs) {
		return minutes > 0 ? `${hours} ч ${minutes} мин` : `${hours} ч`;
	}

	const days = countOf(Math.floor(ms / dayMs), 'день', 'дня', 'дней');

	return hours > 0 ? `${days} ${hours} ч` : days;
}

/** Срок до момента прогноза: "через 4 ч 10 мин"; меньше минуты и наступивший момент - "скоро". */
export function untilLabel(at: string, now: Date): string {
	const left = new Date(at).getTime() - now.getTime();

	return left >= minuteMs ? `через ${formatDuration(left)}` : 'скоро';
}

/** Момент прогноза: "сегодня в 21:40", "завтра в 07:08", "16 октября в 00:51". */
export function momentLabel(at: string, now: Date): string {
	const date = new Date(at);

	if (isSameDay(date, now)) {
		return `сегодня в ${formatTime(date)}`;
	}

	if (isSameDay(date, addDays(now, 1))) {
		return `завтра в ${formatTime(date)}`;
	}

	return formatLongDateTime(date);
}

/** Момент не позже чем через сутки: блюдо пора спасать. Порог тот же, что у истекающих партий. */
export function isSoon(at: string, now: Date): boolean {
	return expiryStatus(at, now) !== 'ok';
}

/** Шаг округления расхода: две-три значащие цифры, как их читают на кухне. */
function rateStep(perHour: number, unit: QuantityUnit): number {
	if (unit === 'pcs') {
		return perHour < 10 ? 0.1 : 1;
	}

	return perHour < 1000 ? 1 : 100;
}

/** Расход в час: "около 290 г", "около 135,4 кг", "около 0,5 шт"; совсем малый - "меньше 1 г". */
export function rateLabel(perHour: number, unit: QuantityUnit): string {
	const step = rateStep(perHour, unit);
	const rounded = Math.round(perHour / step) * step;

	return rounded > 0
		? `около ${formatAmount(rounded, unit)} в час`
		: `меньше ${formatAmount(step, unit)} в час`;
}

/** Почему блюдо закончится: остаток продукта, расход или истечение срока партии, расход на порцию. */
export function forecastReason(item: StopListForecast): string {
	const unit = item.limiting_product.unit;
	const left = `осталось ${formatAmount(item.available_quantity, unit)}`;
	const why =
		item.cause === 'expiry'
			? 'истечёт срок партии'
			: rateLabel(item.consumption_per_hour, unit);

	return `${left} · ${why} · ${formatAmount(item.required_quantity, unit)} на порцию`;
}
