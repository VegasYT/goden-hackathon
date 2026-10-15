import {
	type AnyRoute,
	createMemoryHistory,
	createRouter,
	RouterProvider,
} from '@tanstack/react-router';
import { act, screen, within } from '@testing-library/react';

import type { Point } from '@entities/organization';
import {
	meQueryOptions,
	type Profile,
	type ProfilePoint,
} from '@entities/profile';
import {
	permissions,
	SessionProvider,
	type Permission,
	type Role,
	type Session,
} from '@shared/auth';
import { mockWideScreen, renderWithTheme } from '@shared/lib/test-utils';

import { navItems } from './layouts/app-shell/nav-items';
import { createQueryClient } from './query-client';
import { appRouterOptions } from './router';
import { routeTree } from './routeTree.gen';

export const testSession: Session = {
	userId: 'user-1',
	name: 'Менеджер Тестовый',
};

/** Точка владельца компании «Демо» со всеми правами; поля можно переопределить. */
export function testPoint(overrides: Partial<ProfilePoint> = {}): ProfilePoint {
	return {
		id: 'lenina',
		company_id: 'demo',
		company_name: 'Демо',
		name: 'Точка на Ленина',
		active: true,
		owner: true,
		permissions: [...permissions],
		...overrides,
	};
}

/** Права ролей сотрудника на точке, как их выдаёт сервер. */
export const rolePermissions: Record<Role, Permission[]> = {
	manager: [
		'point.view',
		'staff.view',
		'staff.manage',
		'menu.manage',
		'notifications.manage',
		'catalog.view',
		'deliveries.view',
		'deliveries.register',
		'deliveries.approve',
		'orders.view',
		'orders.cancel',
		'inventory.view',
		'inventory.edit',
		'kitchen.view',
		'analytics.view',
		'recommendations.view',
	],
	approver: [
		'point.view',
		'staff.view',
		'catalog.view',
		'deliveries.view',
		'deliveries.approve',
		'inventory.view',
	],
	cook: ['point.view', 'staff.view', 'kitchen.view', 'catalog.view'],
	waiter: [
		'point.view',
		'staff.view',
		'orders.accept',
		'orders.cancel',
		'orders.view',
		'kitchen.view',
		'catalog.view',
		'recommendations.view',
	],
};

/** Точка `lenina`, где пользователь — сотрудник с ролью и её правами; поля можно переопределить. */
export function testStaffPoint(
	role: Role,
	overrides: Partial<ProfilePoint> = {}
): ProfilePoint {
	return testPoint({
		owner: false,
		role,
		permissions: rolePermissions[role],
		...overrides,
	});
}

/** Управляющий точки `lenina`: всё, кроме оформления заказов, каталога компании и генератора. */
export const manager = testStaffPoint('manager');

/** Приёмщик точки `lenina`: поставки и склад без правок остатков. */
export const approver = testStaffPoint('approver');

/** Повар точки `lenina`: кухня и каталог. */
export const cook = testStaffPoint('cook');

/** Официант точки `lenina`: оформление, история, предложения и кухня. */
export const waiter = testStaffPoint('waiter');

/** Точка компании «Демо», как её отдаёт API организации; поля можно переопределить. */
export function testOrgPoint(overrides: Partial<Point> = {}): Point {
	return {
		id: 'lenina',
		company_id: 'demo',
		name: 'Точка на Ленина',
		active: true,
		...overrides,
	};
}

/** Профиль с заданными точками; компания «Демо» принадлежит пользователю. */
export function testProfile(
	points: ProfilePoint[] = [testPoint()],
	companies: Profile['companies'] = [
		{ id: 'demo', name: 'Демо', owner_id: testSession.userId, active: true },
	]
): Profile {
	return {
		id: testSession.userId,
		email: 'anna@goden.local',
		name: testSession.name,
		companies,
		points,
	};
}

interface RenderAppOptions {
	profile?: Profile;
	tree?: AnyRoute;
	/** Широкий экран ноутбука; `false` оставляет ширину, заданную тестом, по умолчанию планшет. */
	wide?: boolean;
}

/** Рендерит приложение целиком на заданном адресе с профилем в кэше, для тестов. */
export async function renderApp(
	path: string,
	{
		profile = testProfile(),
		tree = routeTree,
		wide = true,
	}: RenderAppOptions = {}
) {
	if (wide) {
		mockWideScreen();
	}

	const queryClient = createQueryClient({ test: true });

	queryClient.setQueryData(meQueryOptions.queryKey, profile);

	const router = createRouter({
		...appRouterOptions({ queryClient, session: testSession }),
		routeTree: tree,
		history: createMemoryHistory({ initialEntries: [path] }),
	});
	const view = renderWithTheme(
		<SessionProvider session={testSession}>
			<RouterProvider router={router} />
		</SessionProvider>
	);

	await act(() => router.load());
	return { router, queryClient, ...view };
}

/** Подписи разделов в основной навигации в порядке пунктов. */
export function navSections() {
	const nav = screen.getByRole('navigation', { name: 'Основная навигация' });

	return navItems
		.map((item) => item.label)
		.filter((label) => within(nav).queryByRole('link', { name: label }));
}
