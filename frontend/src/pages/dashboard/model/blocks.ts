import { can, type ProfilePoint } from '@entities/profile';

/** Какие блоки сводки видит пользователь на точке. */
export interface DashboardBlocks {
	expiring: boolean;
	stopList: boolean;
	hot: boolean;
	deliveries: boolean;
}

export function dashboardBlocks(point: ProfilePoint): DashboardBlocks {
	return {
		expiring: can(point, 'inventory.view'),
		stopList: can(point, 'kitchen.view'),
		hot: can(point, 'recommendations.view'),
		deliveries: can(point, 'deliveries.view'),
	};
}
