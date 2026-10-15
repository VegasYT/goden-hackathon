import type { Delivery, DeliverySummary } from '@entities/delivery';
import { stoppedDishes, type DishAvailability } from '@entities/dish';
import type { ExpiringBatch } from '@entities/stock';
import { formatDateTime } from '@shared/lib/datetime';
import { isExpired, remainingLabel } from '@shared/lib/expiry';
import { countOf, plural } from '@shared/lib/plural';

/** `ok` — всё в порядке, остальное — насколько число требует внимания. */
export type TileTone = 'ok' | 'neutral' | 'warning' | 'error';

/** Что показывает счётчик: число, слово к нему, подпись под ним и важность. */
export interface Tile {
	value: number;
	unit: string;
	note: string;
	tone: TileTone;
}

interface Page<T> {
	items: T[];
	total: number;
}

export function expiringTile(page: Page<ExpiringBatch>, now: Date): Tile {
	const unit = plural(page.total, 'партия', 'партии', 'партий');
	const [nearest] = page.items;

	if (!nearest) {
		return { value: 0, unit, note: 'за сутки ничего не истекает', tone: 'ok' };
	}

	// Список идёт от ближайшего срока, поэтому просроченные партии стоят в его начале.
	const expired = page.items.filter((item) =>
		isExpired(item.batch.expires_at, now)
	).length;

	if (expired > 0) {
		return {
			value: page.total,
			unit,
			note: `${expired} ${plural(expired, 'просрочена', 'просрочены', 'просрочены')}, нужно списать`,
			tone: 'error',
		};
	}

	return {
		value: page.total,
		unit,
		note: `ближайшая: ${remainingLabel(nearest.batch.expires_at, now)}`,
		tone: 'warning',
	};
}

export function stopListTile(menu: DishAvailability[]): Tile {
	const stopped = stoppedDishes(menu);
	const unit = plural(stopped.length, 'блюдо', 'блюда', 'блюд');

	if (menu.length === 0) {
		return { value: 0, unit, note: 'в меню точки нет блюд', tone: 'neutral' };
	}

	if (stopped.length === 0) {
		return {
			value: 0,
			unit,
			note: 'всё меню можно приготовить',
			tone: 'ok',
		};
	}

	return {
		value: stopped.length,
		unit,
		note: `из ${countOf(menu.length, 'блюда', 'блюд', 'блюд')} меню`,
		tone: 'error',
	};
}

export function hotTile(total: number): Tile {
	const unit = plural(total, 'блюдо', 'блюда', 'блюд');

	return total === 0
		? { value: 0, unit, note: 'за 3 дня ничего не истекает', tone: 'ok' }
		: {
				value: total,
				unit,
				note: 'продукты истекают за 3 дня',
				tone: 'warning',
			};
}

export function pendingTile(page: Page<Delivery>, now: Date): Tile {
	const unit = plural(page.total, 'поставка', 'поставки', 'поставок');
	const [latest] = page.items;

	return latest
		? {
				value: page.total,
				unit,
				note: `последняя ${formatDateTime(latest.delivered_at, now)}`,
				tone: 'neutral',
			}
		: { value: 0, unit, note: 'поставок на приёмке нет', tone: 'ok' };
}

export function quarantineTile(
	summary: DeliverySummary,
	deliveries: number
): Tile {
	const items = summary.open_quarantine_items;
	const unit = plural(items, 'позиция', 'позиции', 'позиций');

	return items === 0
		? { value: 0, unit, note: 'решений не ждёт', tone: 'ok' }
		: {
				value: items,
				unit,
				note: `в ${countOf(deliveries, 'поставке', 'поставках', 'поставках')}`,
				tone: 'warning',
			};
}
