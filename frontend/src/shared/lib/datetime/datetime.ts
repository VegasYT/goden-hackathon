const shortDate = new Intl.DateTimeFormat('ru-RU', {
	day: '2-digit',
	month: '2-digit',
});
const fullDate = new Intl.DateTimeFormat('ru-RU', {
	day: '2-digit',
	month: '2-digit',
	year: 'numeric',
});
const clock = new Intl.DateTimeFormat('ru-RU', {
	hour: '2-digit',
	minute: '2-digit',
});
const dayAndMonth = new Intl.DateTimeFormat('ru-RU', {
	day: 'numeric',
	month: 'long',
});
const shortDay = new Intl.DateTimeFormat('ru-RU', {
	day: 'numeric',
	month: 'short',
});

export const minuteMs = 60_000;
export const hourMs = 60 * minuteMs;
export const dayMs = 24 * hourMs;

/** Окна в часах для выбора «24 часа / 3 дня / 7 дней»: сроки партий, горячие предложения, прогноз. */
export const hourWindows = [24, 72, 168] as const;

export type HourWindow = (typeof hourWindows)[number];

/** Подписи окон; заголовок «За <подпись>» читается без правок. */
export const hourWindowLabels: Record<HourWindow, string> = {
	24: '24 часа',
	72: '3 дня',
	168: '7 дней',
};

/** Один и тот же календарный день по часам браузера. */
export function isSameDay(a: string | Date, b: string | Date): boolean {
	return new Date(a).toDateString() === new Date(b).toDateString();
}

/** Время «14:32». */
export function formatTime(value: string | Date): string {
	return clock.format(new Date(value));
}

/** «09.10, 22:00»; дата другого года — «01.04.2027» без времени. */
export function formatDateTime(value: string | Date, now = new Date()): string {
	const date = new Date(value);

	return date.getFullYear() === now.getFullYear()
		? `${shortDate.format(date)}, ${clock.format(date)}`
		: fullDate.format(date);
}

/** Время события: сегодня — «14:38», в другой день — как `formatDateTime`. */
export function formatEventTime(
	value: string | Date,
	now = new Date()
): string {
	return isSameDay(value, now) ? formatTime(value) : formatDateTime(value, now);
}

/** Дата «12.10». */
export function formatShortDate(value: string | Date): string {
	return shortDate.format(new Date(value));
}

/** Дата «12.10», другого года — «12.10.2025». */
export function formatDate(value: string | Date, now = new Date()): string {
	const date = new Date(value);

	return date.getFullYear() === now.getFullYear()
		? shortDate.format(date)
		: fullDate.format(date);
}

/** День «8 окт.» для делений шкалы и подписей графика. */
export function formatShortDay(value: string | Date): string {
	return shortDay.format(new Date(value));
}

/** Дни от и до: «8–14 октября», «15 сентября – 14 октября»; год появляется, если он разный. */
export function formatDayRange(start: Date, end: Date): string {
	return dayAndMonth.formatRange(start, end);
}

/** «10 октября в 09:40». */
export function formatLongDateTime(value: string | Date): string {
	const date = new Date(value);

	return `${dayAndMonth.format(date)} в ${clock.format(date)}`;
}

/** Часовой пояс браузера, имя IANA («Europe/Moscow»): другого источника пояса у клиента нет. */
export function browserTimeZone(): string {
	return Intl.DateTimeFormat().resolvedOptions().timeZone;
}
