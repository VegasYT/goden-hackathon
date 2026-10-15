import type { HotOfferChange, HotOfferSetting } from '@entities/dish';
import { browserTimeZone } from '@shared/lib/datetime';

/** Скрытие до конца текущего дня в часовом поясе браузера. */
export function hideChange(): HotOfferChange {
	return { mode: 'hidden', time_zone: browserTimeZone() };
}

export const autoChange: HotOfferChange = { mode: 'auto' };

export function pinChange(comment: string): HotOfferChange {
	return { mode: 'pinned', comment: comment.trim() };
}

/** Режим, который вернёт отмена: настройка блюда до действия, без настройки — расчёт. */
export function revertChange(
	settings: HotOfferSetting[] | undefined,
	dishId: string
): HotOfferChange {
	const previous = settings?.find(({ dish }) => dish.id === dishId);

	if (previous?.mode === 'pinned') {
		return pinChange(previous.comment);
	}

	return previous?.mode === 'hidden' ? hideChange() : autoChange;
}
