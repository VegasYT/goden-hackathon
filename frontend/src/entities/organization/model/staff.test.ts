import { describe, expect, it } from 'vitest';

import { mayManageStaff, sortStaff } from './staff';

describe('сотрудники точки', () => {
	it('идут по старшинству роли, внутри роли — по имени', () => {
		const sorted = sortStaff([
			{ name: 'Сергей', role: 'waiter' },
			{ name: 'Ольга', role: 'cook' },
			{ name: 'Наталья', role: 'manager' },
			{ name: 'Дмитрий', role: 'cook' },
			{ name: 'Илья', role: 'manager' },
		] as const);

		expect(sorted.map(({ name }) => name)).toEqual([
			'Илья',
			'Наталья',
			'Дмитрий',
			'Ольга',
			'Сергей',
		]);
	});

	it('управляющих меняет только владелец', () => {
		const manager = { name: 'Наталья', role: 'manager' } as const;
		const cook = { name: 'Ольга', role: 'cook' } as const;

		expect(mayManageStaff(true, manager)).toBe(true);
		expect(mayManageStaff(false, manager)).toBe(false);
		expect(mayManageStaff(false, cook)).toBe(true);
	});
});
