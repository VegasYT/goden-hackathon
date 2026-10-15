import { useEffect, useState } from 'react';

import { dayMs, hourMs, minuteMs } from '@shared/lib/datetime';
import { countOf } from '@shared/lib/plural';

/** `expired` — срок истёк, `soon` — осталось не больше суток, `ok` — дольше. */
type ExpiryStatus = 'expired' | 'soon' | 'ok';

function time(value: string | Date): number {
	return new Date(value).getTime();
}

/** Срок истёк, если «годен до» не позже момента проверки. */
export function isExpired(expiresAt: string | Date, now: Date): boolean {
	return time(expiresAt) <= now.getTime();
}

export function expiryStatus(
	expiresAt: string | Date,
	now: Date
): ExpiryStatus {
	const left = time(expiresAt) - now.getTime();

	if (left <= 0) {
		return 'expired';
	}

	return left <= dayMs ? 'soon' : 'ok';
}

/** Оставшийся срок: «ещё 40 мин», «ещё 17 ч», «ещё 3 дня», «ещё 2 месяца»; у истёкшего — пустая строка. */
export function remainingLabel(expiresAt: string | Date, now: Date): string {
	const left = time(expiresAt) - now.getTime();

	if (left <= 0) {
		return '';
	}

	if (left < hourMs) {
		return `ещё ${Math.max(1, Math.floor(left / minuteMs))} мин`;
	}

	if (left < dayMs) {
		return `ещё ${Math.floor(left / hourMs)} ч`;
	}

	const days = Math.floor(left / dayMs);

	if (days < 30) {
		return `ещё ${countOf(days, 'день', 'дня', 'дней')}`;
	}

	if (days < 365) {
		return `ещё ${countOf(Math.floor(days / 30), 'месяц', 'месяца', 'месяцев')}`;
	}

	return `ещё ${countOf(Math.floor(days / 365), 'год', 'года', 'лет')}`;
}

/** Текущее время, которое обновляется раз в минуту. */
export function useNow(): Date {
	const [now, setNow] = useState(() => new Date());

	useEffect(() => {
		const id = window.setInterval(() => setNow(new Date()), minuteMs);

		return () => window.clearInterval(id);
	}, []);

	return now;
}
