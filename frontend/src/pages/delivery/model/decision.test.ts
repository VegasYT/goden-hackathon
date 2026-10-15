import { describe, expect, it } from 'vitest';

import type { DeliveryItem } from '@entities/delivery';
import { testDeliveryItem } from '@entities/delivery/index.testing';

import {
	approvalBody,
	decisionProblems,
	expireDecision,
	initialDecision,
	type Decision,
} from './decision';

const now = new Date(2026, 9, 12, 14, 32);

function item(overrides: Partial<DeliveryItem> = {}): DeliveryItem {
	return testDeliveryItem({
		id: 'cream',
		product_id: 'p-cream',
		product_name: 'Сливки 33%',
		unit: 'ml',
		declared_quantity: 20000,
		expires_at: new Date(2026, 9, 18, 23, 59).toISOString(),
		...overrides,
	});
}

const decided = (overrides: Partial<Decision>): Decision => ({
	...initialDecision(item(), now),
	...overrides,
});

describe('начальное решение', () => {
	it('непросроченная позиция принимается целиком и сразу решена', () => {
		const decision = initialDecision(item(), now);

		expect(decision).toMatchObject({ accepted: 20000, rejected: 0 });
		expect(decisionProblems(item(), decision)).toEqual([]);
	});

	it('просроченная позиция отклоняется с причиной и комментарием о сроке', () => {
		const spinach = item({
			declared_quantity: 3000,
			expires_at: new Date(2026, 9, 9, 22, 0).toISOString(),
		});
		const decision = initialDecision(spinach, now);

		expect(decision).toMatchObject({
			accepted: 0,
			rejected: 3000,
			rejectReason: 'expired',
			autoReason: true,
			comment: 'Срок годности истёк 09.10, 22:00',
		});
		expect(decisionProblems(spinach, decision)).toEqual([]);
	});
});

describe('проблемы позиции', () => {
	it.each<[string, Partial<Decision>, string[]]>([
		[
			'все поля пустые',
			{ accepted: null, rejected: null, quarantined: null },
			['unallocated'],
		],
		[
			'сверх заявленного',
			{ accepted: 20000, rejected: 1000 },
			['over', 'reject-reason', 'comment'],
		],
		[
			'отклонение с причиной, но без комментария',
			{
				accepted: 18000,
				rejected: 2000,
				rejectReason: 'physical_damage',
				comment: '  ',
			},
			['comment'],
		],
		[
			'карантин без причины',
			{ accepted: 18000, quarantined: 2000, comment: '+9 °C' },
			['quarantine-reason'],
		],
		[
			'принято у просроченной',
			{ expired: true, accepted: 20000 },
			['accepted-expired'],
		],
	])('%s', (_, overrides, problems) => {
		expect(decisionProblems(item(), decided(overrides))).toEqual(problems);
	});
});

describe('позиция истекает во время заполнения', () => {
	it('принятое обнуляется, прежнее значение запоминается, нужно распределить заново', () => {
		const expired = expireDecision(
			decided({ accepted: 15000, rejected: 5000 })
		);

		expect(expired).toMatchObject({
			expired: true,
			accepted: 0,
			previousAccepted: 15000,
		});
		expect(decisionProblems(item(), expired)).toContain('unallocated');
	});
});

describe('тело приёмки', () => {
	it('причины и комментарий только у ненулевых частей, комментарий без пробелов по краям', () => {
		const body = approvalBody(
			[item(), item({ id: 'eggs', unit: 'pcs', declared_quantity: 360 })],
			[
				decided({
					accepted: 18000,
					rejected: 2000,
					rejectReason: 'physical_damage',
					quarantineReason: 'storage_violation',
					comment: ' 2 пакета вздуты ',
				}),
				{
					...initialDecision(item({ declared_quantity: 360 }), now),
					comment: 'лишнее',
				},
			]
		);

		expect(body.items).toEqual([
			{
				item_id: 'cream',
				accepted_quantity: 18000,
				rejected_quantity: 2000,
				quarantined_quantity: 0,
				reject_reason: 'physical_damage',
				quarantine_reason: undefined,
				comment: '2 пакета вздуты',
			},
			{
				item_id: 'eggs',
				accepted_quantity: 360,
				rejected_quantity: 0,
				quarantined_quantity: 0,
				reject_reason: undefined,
				quarantine_reason: undefined,
				comment: undefined,
			},
		]);
	});
});
