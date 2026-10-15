import { describe, expect, it } from 'vitest';

import type {
	Delivery,
	DeliveryHistoryEntry,
	DeliveryItem,
} from '@entities/delivery';
import {
	testDelivery,
	testDeliveryItem,
} from '@entities/delivery/index.testing';

import { distributionChange, historyView } from './history';

function item(overrides: Partial<DeliveryItem> = {}): DeliveryItem {
	return testDeliveryItem({
		id: 'mascarpone',
		product_id: 'product-mascarpone',
		product_name: 'Маскарпоне',
		unit: 'g',
		declared_quantity: 6000,
		expires_at: '2026-10-25T23:59:00Z',
		...overrides,
	});
}

function state(...items: DeliveryItem[]): Delivery {
	return testDelivery({ status: 'approved', items });
}

const anna = { type: 'user' as const, id: 'anna', name: 'Анна Котова' };
const generator = { type: 'service' as const, id: 'goden-generator' };

function entry(
	type: DeliveryHistoryEntry['type'],
	occurredAt: string,
	delivery: Delivery,
	extra: Partial<DeliveryHistoryEntry> = {}
): DeliveryHistoryEntry {
	return { type, occurred_at: occurredAt, actor: anna, delivery, ...extra };
}

const flour = (accepted: number, rejected: number) =>
	item({
		id: 'flour',
		product_name: 'Мука',
		declared_quantity: 25000,
		accepted_quantity: accepted,
		rejected_quantity: rejected,
	});

const approved = state(
	item({ accepted_quantity: 4000, quarantined_quantity: 2000 }),
	flour(25000, 0)
);

describe('historyView', () => {
	it('склеивает принятую и отклонённую части одного решения по карантину', () => {
		const accepted = state(
			item({ accepted_quantity: 5500, quarantined_quantity: 500 }),
			flour(25000, 0)
		);
		const rejected = state(
			item({ accepted_quantity: 5500, rejected_quantity: 500 }),
			flour(25000, 0)
		);
		const at = '2026-10-12T12:30:00Z';

		const views = historyView([
			entry('registered', '2026-10-10T09:40:00Z', approved, {
				actor: generator,
			}),
			entry('approved', '2026-10-11T14:41:00Z', approved),
			entry('quarantine_accepted', at, accepted, { comment: 'подтекла' }),
			entry('quarantine_rejected', at, rejected, {
				reject_reason: 'storage_violation',
				comment: 'подтекла',
			}),
		]);

		expect(views.map((view) => view.kind)).toEqual([
			'registered',
			'approved',
			'quarantine',
		]);
		expect(views[0].actor).toEqual(generator);
		expect(views[1]).toMatchObject({
			outcomes: { toStock: 2, rejected: 0, quarantined: 1 },
		});
		expect(views[2]).toMatchObject({
			item: { id: 'mascarpone' },
			accepted: 1500,
			rejected: 500,
			rejectReason: 'storage_violation',
			comment: 'подтекла',
		});
	});

	it('решение с одной частью — одно событие только с ней', () => {
		const views = historyView([
			entry('approved', '2026-10-11T14:41:00Z', approved),
			entry(
				'quarantine_rejected',
				'2026-10-12T12:30:00Z',
				state(
					item({ accepted_quantity: 4000, rejected_quantity: 2000 }),
					flour(25000, 0)
				),
				{ reject_reason: 'quality_issue' }
			),
		]);

		expect(views[1]).toMatchObject({ accepted: 0, rejected: 2000 });
	});

	it('исправление показывает только изменённые позиции с изменением склада', () => {
		const views = historyView([
			entry('approved', '2026-10-11T14:41:00Z', approved),
			entry(
				'corrected',
				'2026-10-12T10:05:00Z',
				state(
					item({ accepted_quantity: 4000, quarantined_quantity: 2000 }),
					flour(20000, 5000)
				),
				{ reason: 'Пересчитали после разгрузки' }
			),
		]);

		expect(views[1]).toMatchObject({
			kind: 'corrected',
			reason: 'Пересчитали после разгрузки',
			changes: [
				{
					item: { id: 'flour' },
					change: {
						parts: [
							{ part: 'accepted', before: 25000, after: 20000 },
							{ part: 'rejected', before: 0, after: 5000 },
						],
						stock: -5000,
					},
				},
			],
		});
	});
});

describe('distributionChange', () => {
	it('рост принятого — новая партия', () => {
		expect(
			distributionChange(
				{
					accepted_quantity: 18000,
					rejected_quantity: 2000,
					quarantined_quantity: 0,
				},
				{
					accepted_quantity: 19000,
					rejected_quantity: 1000,
					quarantined_quantity: 0,
				}
			).stock
		).toBe(1000);
	});
});
