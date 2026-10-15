import { describe, expect, it } from 'vitest';

import type { Movement } from '../api/stock';
import {
	movementBatches,
	movementDays,
	movementDelta,
	movementDetails,
	movementTitle,
} from './history';

const now = new Date(2026, 9, 12, 14, 32);

function movement(overrides: Partial<Movement> = {}): Movement {
	return {
		id: 'm-1',
		type: 'consumed',
		occurred_at: new Date(2026, 9, 12, 14, 20).toISOString(),
		actor: { type: 'service', id: 'goden-generator' },
		changes: [{ batch_id: 'b-111', delivery_number: '111', delta: -120 }],
		...overrides,
	};
}

describe('история продукта', () => {
	it('группирует движения по дням: сегодня, вчера, другой год', () => {
		const days = movementDays(
			[
				movement({ id: 'a' }),
				movement({
					id: 'b',
					occurred_at: new Date(2026, 9, 12, 10, 5).toISOString(),
				}),
				movement({
					id: 'c',
					occurred_at: new Date(2026, 9, 11, 8, 10).toISOString(),
				}),
				movement({
					id: 'd',
					occurred_at: new Date(2025, 11, 30, 9, 0).toISOString(),
				}),
			],
			now
		);

		expect(days.map((day) => [day.label, day.movements.length])).toEqual([
			['Сегодня, 12.10', 2],
			['11.10', 1],
			['30.12.2025', 1],
		]);
	});

	it('расход из двух партий: сумма изменений и обе партии', () => {
		const split = movement({
			changes: [
				{ batch_id: 'b-111', delivery_number: '111', delta: -100 },
				{ batch_id: 'b-113', delivery_number: '113', delta: -20 },
			],
		});

		expect(movementTitle(split)).toBe('Расход по заказу');
		expect(movementDelta(split)).toBe(-120);
		expect(movementBatches(split)).toBe('партии № 111, № 113');
		expect(movementDetails(split, now)).toBeUndefined();
	});

	it.each([
		[
			movement({
				type: 'written_off',
				reason: 'physical_damage',
				comment: 'Помялись при разгрузке',
			}),
			'Списание',
			'Повреждена упаковка · «Помялись при разгрузке»',
		],
		[
			movement({ type: 'adjusted', reason: 'Пересчёт', comment: undefined }),
			'Исправление остатка',
			'Пересчёт',
		],
		[
			movement({
				type: 'received',
				expires_at: new Date(2026, 9, 17, 23, 59).toISOString(),
			}),
			'Получение партии',
			'годна до 17.10, 23:59',
		],
		[
			movement({
				type: 'expiry_warning',
				changes: [{ batch_id: 'b-111', delivery_number: '111', delta: 0 }],
				expires_at: new Date(2026, 9, 13, 8, 0).toISOString(),
			}),
			'Предупреждение о сроке',
			'Годна до 13.10, 08:00: срок подходит к концу',
		],
		[
			movement({
				type: 'expired',
				changes: [{ batch_id: 'b-111', delivery_number: '111', delta: 0 }],
			}),
			'Срок истёк',
			'Остаток просрочен и ждёт списания',
		],
	])('%#: подпись и подробности', (item, title, details) => {
		expect(movementTitle(item)).toBe(title);
		expect(movementDetails(item, now)).toBe(details);
		expect(movementBatches(item)).toBe('партия № 111');
	});
});
