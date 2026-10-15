import { useQuery } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { can, usePoint } from '@entities/profile';
import { useNow } from '@shared/lib/expiry';
import { TabLabel } from '@shared/ui/count-badge';
import { SectionTabs, TabLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';

import {
	reorderCountQueryOptions,
	urgentExpiringQueryOptions,
} from '../api/tab-counts';
import { HistoryPanel } from './history-panel';
import { useHistoryPanel } from './use-history-panel';

export type StockTab = 'levels' | 'expiring' | 'reorder';

interface StockLayoutProps {
	pointId: string;
	tab: StockTab;
	history: string | undefined;
	children: ReactNode;
}

/** Склад точки: вкладки «Остатки», «Сроки», «Пора заказать» и история продукта поверх любой из них. */
export function StockLayout({
	pointId,
	tab,
	history,
	children,
}: StockLayoutProps) {
	const now = useNow();
	const { point } = usePoint(pointId);
	const mayReorder = can(point, 'recommendations.view');
	const urgent = useQuery(urgentExpiringQueryOptions(pointId));
	const reorder = useQuery({
		...reorderCountQueryOptions(pointId),
		enabled: mayReorder,
	});
	const openHistory = useHistoryPanel();

	return (
		<PageContent>
			<PageHeader title='Склад' meta={point.name} />
			<SectionTabs value={tab} label='Разделы склада'>
				<TabLink
					value='levels'
					label='Остатки'
					to='/points/$pointId/stock'
					params={{ pointId }}
				/>
				<TabLink
					value='expiring'
					label={
						<TabLabel text='Сроки' count={urgent.data?.total} tone='warning' />
					}
					to='/points/$pointId/stock/expiring'
					params={{ pointId }}
				/>
				{mayReorder && (
					<TabLink
						value='reorder'
						label={
							<TabLabel text='Пора заказать' count={reorder.data?.total} />
						}
						to='/points/$pointId/stock/reorder'
						params={{ pointId }}
					/>
				)}
			</SectionTabs>
			{children}
			<HistoryPanel
				pointId={pointId}
				companyId={point.company_id}
				productId={history}
				now={now}
				onClose={() => openHistory(undefined)}
			/>
		</PageContent>
	);
}
