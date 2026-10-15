import type { QueryClient } from '@tanstack/react-query';
import { notFound } from '@tanstack/react-router';

import {
	can,
	findPoint,
	meQueryOptions,
	type ProfilePoint,
} from '@entities/profile';
import type { Permission } from '@shared/auth';

import { canOpen, navItems, type NavItem } from './layouts/app-shell/nav-items';

export class ForbiddenError extends Error {
	readonly section: string;

	constructor(section: string) {
		super(`Нет доступа к разделу «${section}»`);
		this.name = 'ForbiddenError';
		this.section = section;
	}
}

/** Точки из адреса нет в профиле пользователя: чужая, несуществующая или его сняли с неё. */
export class PointUnavailableError extends Error {
	constructor() {
		super('Точка недоступна');
		this.name = 'PointUnavailableError';
	}
}

/** Вызывается в `beforeLoad` роута точки: отдаёт точку из профиля или бросает `PointUnavailableError`. */
export async function requirePoint(
	queryClient: QueryClient,
	pointId: string
): Promise<ProfilePoint> {
	const profile = await queryClient.ensureQueryData(meQueryOptions);
	const point = findPoint(profile, pointId);

	if (!point) {
		throw new PointUnavailableError();
	}

	return point;
}

/** Вызывается в `beforeLoad` роута раздела: выключенный в сборке раздел не найден, без права на точке — `ForbiddenError`. */
export function requireSection(point: ProfilePoint, to: NavItem['to']) {
	const item = navItems.find((navItem) => navItem.to === to);

	if (item?.enabled?.() === false) {
		throw notFound();
	}

	if (item && !canOpen(point, item)) {
		throw new ForbiddenError(item.label);
	}
}

/** Вызывается в `beforeLoad` страницы внутри раздела: без права на точке — `ForbiddenError` с подписью `section`. */
export function requirePermission(
	point: ProfilePoint,
	permission: Permission,
	section: string
) {
	if (!can(point, permission)) {
		throw new ForbiddenError(section);
	}
}
