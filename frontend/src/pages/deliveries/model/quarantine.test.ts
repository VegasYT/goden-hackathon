import { describe, expect, it } from 'vitest';

import type { Delivery, DeliveryItem } from '@entities/delivery';
import {
	testDelivery,
	testDeliveryItem,
} from '@entities/delivery/index.testing';

import {
	closedQuarantineMessage,
	initialResolution,
	openQuarantineItems,
	quarantineAge,
	resolutionBody,
	resolutionOutcome,
	resolutionProblems,
	type Resolution,
} from './quarantine';

const now = new Date('2026-10-12T12:00:00Z');

function item(overrides: Partial<DeliveryItem> = {}): DeliveryItem {
	return testDeliveryItem({
		id: 'mozzarella',
		product_id: 'product-mozzarella',
		product_name: 'Моцарелла',
		unit: 'g',
		declared_quantity: 5000,
		expires_at: '2026-10-19T23:59:00Z',
		accepted_quantity: 3500,
		quarantined_quantity: 1500,
		...overrides,
	});
}

function delivery(id: string, items: DeliveryItem[]): Delivery {
	return testDelivery({
		id,
		number: id,
		delivered_at: '2026-10-08T10:00:00Z',
		status: 'approved',
		items,
	});
}

const resolution = (overrides: Partial<Resolution>): Resolution => ({
	...initialResolution(item(), now),
	...overrides,
});

describe('openQuarantineItems', () => {
	it('берёт только позиции с карантином без решения, первыми ждущие дольше', () => {
		const entries = openQuarantineItems([
			delivery('116', [
				item({ id: 'mozzarella', quarantined_at: '2026-10-11T12:00:00Z' }),
				item({ id: 'parmesan', quarantined_quantity: 0 }),
			]),
			delivery('114', [
				item({ id: 'shrimps', quarantined_at: '2026-10-10T11:00:00Z' }),
			]),
		]);

		expect(entries.map((entry) => entry.item.id)).toEqual([
			'shrimps',
			'mozzarella',
		]);
		expect(quarantineAge(entries[0], now)).toBe('в карантине 2 дня');
	});
});

describe('initialResolution', () => {
	it('предлагает принять всё количество в карантине', () => {
		expect(initialResolution(item(), now)).toMatchObject({
			accepted: 1500,
			rejected: 0,
			expired: false,
		});
	});

	it('у просроченной позиции отклоняет всё с причиной и комментарием о сроке', () => {
		const expired = item({ expires_at: '2026-10-11T09:00:00Z' });

		expect(initialResolution(expired, now)).toMatchObject({
			accepted: 0,
			rejected: 1500,
			rejectReason: 'expired',
			expired: true,
		});
		expect(initialResolution(expired, now).comment).toMatch(
			/^Срок годности истёк \d\d\.10, \d\d:00$/
		);
	});
});

describe('resolutionProblems', () => {
	it.each<[string, Partial<Resolution>, string[]]>([
		['принять всё', {}, []],
		[
			'сумма меньше карантина',
			{ accepted: 1000, rejected: null },
			['unallocated'],
		],
		['сумма больше карантина', { accepted: 2000, rejected: null }, ['over']],
		[
			'отклонение без причины и комментария',
			{ accepted: 1000, rejected: 500 },
			['reject-reason', 'comment'],
		],
		[
			'отклонение с причиной и комментарием',
			{
				accepted: 1000,
				rejected: 500,
				rejectReason: 'physical_damage',
				comment: 'один блок вскрыт',
			},
			[],
		],
		[
			'принято у просроченной',
			{ expired: true, accepted: 1500, rejected: 0 },
			['accepted-expired'],
		],
	])('%s', (_, overrides, problems) => {
		expect(resolutionProblems(item(), resolution(overrides))).toEqual(problems);
	});
});

describe('resolutionOutcome и тело запроса', () => {
	it.each<[Partial<Resolution>, string]>([
		[{ accepted: 1500, rejected: 0 }, 'accept'],
		[{ accepted: 0, rejected: 1500 }, 'reject'],
		[{ accepted: 1000, rejected: 500 }, 'split'],
	])('%o — %s', (overrides, outcome) => {
		expect(resolutionOutcome(resolution(overrides))).toBe(outcome);
	});

	it('причина и комментарий уходят только при отклонённой части', () => {
		expect(
			resolutionBody(
				resolution({ comment: 'лишний текст', rejectReason: 'other' })
			)
		).toEqual({
			accepted_quantity: 1500,
			rejected_quantity: 0,
			reject_reason: undefined,
			comment: undefined,
		});
		expect(
			resolutionBody(
				resolution({
					accepted: 1000,
					rejected: 500,
					rejectReason: 'physical_damage',
					comment: ' один блок вскрыт ',
				})
			)
		).toEqual({
			accepted_quantity: 1000,
			rejected_quantity: 500,
			reject_reason: 'physical_damage',
			comment: 'один блок вскрыт',
		});
	});
});

describe('closedQuarantineMessage', () => {
	const stale = item({ quarantined_at: '2026-10-11T12:00:00Z' });
	const decided = (decidedAt: string) =>
		item({
			quarantined_quantity: 0,
			quarantine_resolutions: [
				{
					decided_at: decidedAt,
					decided_by: { type: 'service', id: 'goden-generator' },
					accepted_quantity: 1500,
					rejected_quantity: 0,
				},
			],
		});

	it('решение после открытия карантина называет, кто решил', () => {
		expect(
			closedQuarantineMessage(stale, decided('2026-10-12T11:39:00Z'))
		).toMatch(/^Карантин по «Моцарелла» уже решён: Генератор, \d\d:39$/);
	});

	it('без решения после открытия карантин закрыло исправление', () => {
		expect(
			closedQuarantineMessage(stale, decided('2026-10-09T10:00:00Z'))
		).toBe('Карантин по «Моцарелла» уже закрыт исправлением приёмки');
	});
});
