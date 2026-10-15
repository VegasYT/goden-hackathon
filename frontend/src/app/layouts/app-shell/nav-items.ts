import { can, type ProfilePoint } from '@entities/profile';
import type { Permission } from '@shared/auth';
import { generatorEnabled } from '@shared/config';
import type { IconName } from '@shared/ui/icon';

import type { FileRouteTypes } from '../../routeTree.gen';

export interface NavItem {
	to: Extract<FileRouteTypes['to'], `/points/$pointId${string}`>;
	label: string;
	/** Подпись в узкой панели, если полная не помещается. */
	shortLabel?: string;
	icon: IconName;
	/** Пункт активен только на своём адресе, без вложенных путей. */
	exact?: boolean;
	/** Право, без которого раздел недоступен; без поля раздел открыт любому на точке. */
	permission?: Permission;
	/** Включён ли раздел в сборке; без поля раздел есть всегда. */
	enabled?: () => boolean;
	/** Пункт отделён от предыдущих чертой. */
	separated?: boolean;
	/** Подпись перед пунктом: с него начинается группа, в узкой панели вместо подписи черта. */
	heading?: (point: ProfilePoint) => string;
}

export const navItems: NavItem[] = [
	{ to: '/points/$pointId', label: 'Сводка', icon: 'dashboard', exact: true },
	{
		to: '/points/$pointId/stock',
		label: 'Склад',
		icon: 'stock',
		permission: 'inventory.view',
	},
	{
		to: '/points/$pointId/deliveries',
		label: 'Поставки',
		icon: 'deliveries',
		permission: 'deliveries.view',
	},
	{
		to: '/points/$pointId/orders',
		label: 'Заказы',
		icon: 'orders',
		permission: 'orders.view',
	},
	{
		to: '/points/$pointId/kitchen',
		label: 'Кухня',
		icon: 'kitchen',
		permission: 'kitchen.view',
	},
	{
		to: '/points/$pointId/stop-list',
		label: 'Стоп-лист',
		icon: 'stopList',
		permission: 'kitchen.view',
	},
	{
		to: '/points/$pointId/analytics',
		label: 'Аналитика',
		icon: 'analytics',
		permission: 'analytics.view',
	},
	{
		to: '/points/$pointId/staff',
		label: 'Сотрудники',
		icon: 'staff',
		permission: 'staff.view',
	},
	{
		to: '/points/$pointId/settings',
		label: 'Настройки точки',
		shortLabel: 'Настройки',
		icon: 'pointSettings',
		permission: 'notifications.manage',
	},
	{
		to: '/points/$pointId/catalog',
		label: 'Каталог',
		icon: 'catalog',
		permission: 'products.manage',
		heading: (point) => `Компания «${point.company_name}»`,
	},
	{
		to: '/points/$pointId/company',
		label: 'Точки и компания',
		shortLabel: 'Компания',
		icon: 'company',
		permission: 'company.manage',
	},
	{
		to: '/points/$pointId/generator',
		label: 'Генератор',
		icon: 'generator',
		permission: 'generator.manage',
		enabled: generatorEnabled,
		separated: true,
	},
];

export function canOpen(point: ProfilePoint, item: NavItem): boolean {
	return (
		(item.enabled?.() ?? true) &&
		(!item.permission || can(point, item.permission))
	);
}

/** Пункт раздела, в котором находится адрес роута: самый длинный совпавший. */
export function navItemOf(routePath: string): NavItem | undefined {
	const path = routePath.replace(/\/$/, '');

	return navItems
		.filter((item) => path === item.to || path.startsWith(`${item.to}/`))
		.sort((a, b) => b.to.length - a.to.length)[0];
}
