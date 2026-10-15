import { describe, expect, it } from 'vitest';

import type { Profile, ProfilePoint } from '../api/me-query';
import { isPointDisabled, roleLabel, startPoint, writeAccess } from './profile';

function point(id: string, extra: Partial<ProfilePoint> = {}): ProfilePoint {
	return {
		id,
		company_id: 'c1',
		company_name: 'Демо',
		name: `Точка ${id}`,
		active: true,
		owner: false,
		permissions: [],
		...extra,
	};
}

function profile(points: ProfilePoint[], companyActive = true): Profile {
	return {
		id: 'u1',
		email: 'anna@goden.local',
		name: 'Анна',
		companies: [
			{ id: 'c1', name: 'Демо', owner_id: 'u1', active: companyActive },
		],
		points,
	};
}

describe('профиль', () => {
	it('владельцу подписывает «Владелец», сотруднику — роль по-русски', () => {
		expect(roleLabel(point('a', { owner: true }))).toBe('Владелец');
		expect(roleLabel(point('a', { role: 'manager' }))).toBe('Управляющий');
	});

	it('точка отключённой компании считается отключённой', () => {
		const a = point('a');

		expect(isPointDisabled(profile([a], false), a)).toBe(true);
		expect(isPointDisabled(profile([a]), a)).toBe(false);
	});

	it('запись: сначала объясняется отключённая точка, потом право', () => {
		const approver = point('a', { permissions: ['deliveries.approve'] });
		const cook = point('a');

		expect(
			writeAccess(profile([approver]), approver, 'deliveries.approve')
		).toBe('allowed');
		expect(writeAccess(profile([cook]), cook, 'deliveries.approve')).toBe(
			'forbidden'
		);
		expect(
			writeAccess(profile([cook], false), cook, 'deliveries.approve')
		).toBe('disabled');
		expect(writeAccess(profile([]), undefined, 'deliveries.approve')).toBe(
			'forbidden'
		);
	});

	it('без адреса открывает последнюю точку, а если её нет в профиле — первую работающую', () => {
		const p = profile([point('a', { active: false }), point('b'), point('c')]);

		expect(startPoint(p, 'c')?.id).toBe('c');
		expect(startPoint(p, 'gone')?.id).toBe('b');
		expect(startPoint(p, null)?.id).toBe('b');
	});
});
