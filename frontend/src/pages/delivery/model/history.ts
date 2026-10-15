import {
	itemOutcomes,
	type Delivery,
	type DeliveryHistoryEntry,
	type DeliveryItem,
	type Distribution,
	type ItemOutcomes,
} from '@entities/delivery';

import { parts, quantityOf, type Part } from './decision';

interface PartChange {
	part: Part;
	before: number;
	after: number;
}

/** Изменение распределения позиции: части, которые поменялись, и разница принятого на складе. */
export interface DistributionChange {
	parts: PartChange[];
	/** Больше нуля — новая партия на складе, меньше — снято со склада. */
	stock: number;
}

export function distributionChange(
	before: Distribution,
	after: Distribution
): DistributionChange {
	return {
		parts: parts
			.map((part) => ({
				part,
				before: before[quantityOf[part]],
				after: after[quantityOf[part]],
			}))
			.filter((change) => change.before !== change.after),
		stock: after.accepted_quantity - before.accepted_quantity,
	};
}

interface ViewBase {
	at: string;
	actor: DeliveryHistoryEntry['actor'];
	/** Состояние поставки сразу после события. */
	delivery: Delivery;
}

export type HistoryView = ViewBase &
	(
		| { kind: 'registered' }
		| { kind: 'approved'; outcomes: ItemOutcomes }
		| {
				kind: 'quarantine';
				item: DeliveryItem;
				accepted: number;
				rejected: number;
				rejectReason?: string;
				comment?: string;
		  }
		| {
				kind: 'corrected';
				reason?: string;
				changes: { item: DeliveryItem; change: DistributionChange }[];
		  }
	);

type QuarantineView = Extract<HistoryView, { kind: 'quarantine' }>;

function viewBase(entry: DeliveryHistoryEntry): ViewBase {
	return {
		at: entry.occurred_at,
		actor: entry.actor,
		delivery: entry.delivery,
	};
}

function itemIn(delivery: Delivery | undefined, id: string) {
	return delivery?.items.find((item) => item.id === id);
}

/** Решение по карантину: позиция, у которой уменьшился карантин, и сколько из него принято и отклонено. */
function quarantineView(
	entry: DeliveryHistoryEntry,
	previous: Delivery | undefined
): QuarantineView | undefined {
	for (const item of entry.delivery.items) {
		const before = itemIn(previous, item.id);

		if (before && item.quarantined_quantity < before.quarantined_quantity) {
			return {
				kind: 'quarantine',
				...viewBase(entry),
				item,
				accepted: item.accepted_quantity - before.accepted_quantity,
				rejected: item.rejected_quantity - before.rejected_quantity,
				rejectReason: entry.reject_reason,
				comment: entry.comment,
			};
		}
	}

	return undefined;
}

/** Добавляет решение по карантину; вторая часть того же решения дописывается в предыдущее событие. */
function addQuarantine(
	views: HistoryView[],
	entry: DeliveryHistoryEntry,
	previous: Delivery | undefined
) {
	const view = quarantineView(entry, previous);
	const last = views.at(-1);

	if (!view) {
		return;
	}

	if (
		last?.kind === 'quarantine' &&
		last.at === view.at &&
		last.item.id === view.item.id
	) {
		views[views.length - 1] = {
			...last,
			delivery: view.delivery,
			item: view.item,
			accepted: last.accepted + view.accepted,
			rejected: last.rejected + view.rejected,
			rejectReason: last.rejectReason ?? view.rejectReason,
			comment: last.comment ?? view.comment,
		};
	} else {
		views.push(view);
	}
}

/** События истории для показа: принятая и отклонённая части одного решения по карантину склеиваются в одно. */
export function historyView(entries: DeliveryHistoryEntry[]): HistoryView[] {
	const views: HistoryView[] = [];
	let previous: Delivery | undefined;

	for (const entry of entries) {
		const base = viewBase(entry);

		switch (entry.type) {
			case 'registered':
				views.push({ ...base, kind: 'registered' });
				break;

			case 'approved':
				views.push({
					...base,
					kind: 'approved',
					outcomes: itemOutcomes(entry.delivery.items),
				});
				break;

			case 'quarantine_accepted':
				addQuarantine(views, entry, previous);
				break;

			case 'quarantine_rejected':
				addQuarantine(views, entry, previous);
				break;

			case 'corrected':
				views.push({
					...base,
					kind: 'corrected',
					reason: entry.reason,
					changes: entry.delivery.items.flatMap((item) => {
						const before = itemIn(previous, item.id);
						const change = before && distributionChange(before, item);

						return change && change.parts.length > 0 ? [{ item, change }] : [];
					}),
				});
				break;
		}

		previous = entry.delivery;
	}

	return views;
}
