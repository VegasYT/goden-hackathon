import { roleLabels, type Permission } from '@shared/auth';

import type { Profile, ProfilePoint } from '../api/me-query';

export function findPoint(
	profile: Profile,
	pointId: string
): ProfilePoint | undefined {
	return profile.points.find((point) => point.id === pointId);
}

/** Есть ли у пользователя право на точке; без точки права нет. */
export function can(
	point: ProfilePoint | undefined,
	permission: Permission
): boolean {
	return point?.permissions.includes(permission) ?? false;
}

/** Подпись пользователя на точке: «Владелец» или роль по-русски. */
export function roleLabel(point: ProfilePoint): string | undefined {
	if (point.owner) {
		return 'Владелец';
	}

	return point.role && roleLabels[point.role];
}

/** Компания пользователя отключена: её данные только для чтения. */
export function isCompanyDisabled(
	profile: Profile,
	companyId: string
): boolean {
	return profile.companies.find(({ id }) => id === companyId)?.active === false;
}

/** Точка отключена сама или вместе со своей компанией. */
export function isPointDisabled(
	profile: Profile,
	point: ProfilePoint
): boolean {
	return !point.active || isCompanyDisabled(profile, point.company_id);
}

/** Можно ли записывать на точке: `allowed`; `disabled` — точка отключена, это объясняется первым; `forbidden` — нет права. */
export type WriteAccess = 'allowed' | 'disabled' | 'forbidden';

export function writeAccess(
	profile: Profile,
	point: ProfilePoint | undefined,
	permission: Permission
): WriteAccess {
	if (point !== undefined && isPointDisabled(profile, point)) {
		return 'disabled';
	}

	return can(point, permission) ? 'allowed' : 'forbidden';
}

/** Пользователь может записывать на точке: есть право, и точка работает. */
export function mayWrite(
	profile: Profile,
	point: ProfilePoint | undefined,
	permission: Permission
): boolean {
	return writeAccess(profile, point, permission) === 'allowed';
}

interface CompanyPoints {
	companyId: string;
	companyName: string;
	points: ProfilePoint[];
}

/** Точки профиля по компаниям в порядке профиля. */
export function pointsByCompany(profile: Profile): CompanyPoints[] {
	const groups = new Map<string, CompanyPoints>();

	for (const point of profile.points) {
		const group = groups.get(point.company_id) ?? {
			companyId: point.company_id,
			companyName: point.company_name,
			points: [],
		};

		group.points.push(point);
		groups.set(point.company_id, group);
	}

	return [...groups.values()];
}

/** Точка для открытия без адреса: последняя выбранная, иначе первая работающая, иначе первая. */
export function startPoint(
	profile: Profile,
	lastPointId: string | null
): ProfilePoint | undefined {
	const last = lastPointId ? findPoint(profile, lastPointId) : undefined;

	return (
		last ??
		profile.points.find((point) => !isPointDisabled(profile, point)) ??
		profile.points[0]
	);
}

/** Своя работающая компания без точек: в ней первый вход создаёт точку. */
export function companyWithoutPoints(profile: Profile) {
	return profile.companies.find(
		(company) =>
			company.active &&
			!profile.points.some((point) => point.company_id === company.id)
	);
}
