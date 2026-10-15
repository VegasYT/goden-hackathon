/** Права приложения; профиль `/me` отдаёт их для каждой точки пользователя. */
export const permissions = [
	'point.view',
	'staff.view',
	'staff.manage',
	'catalog.view',
	'company.view',
	'company.manage',
	'menu.manage',
	'notifications.manage',
	'deliveries.view',
	'deliveries.register',
	'deliveries.approve',
	'orders.view',
	'orders.accept',
	'orders.cancel',
	'inventory.view',
	'inventory.edit',
	'kitchen.view',
	'products.manage',
	'dishes.manage',
	'analytics.view',
	'recommendations.view',
	'generator.manage',
] as const;

export type Permission = (typeof permissions)[number];
