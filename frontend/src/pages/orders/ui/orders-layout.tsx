import { Box, Button } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import {
	can,
	usePoint,
	writeAccess,
	type WriteAccess,
} from '@entities/profile';
import { TabLabel } from '@shared/ui/count-badge';
import { Icon } from '@shared/ui/icon';
import { ButtonLink, SectionTabs, TabLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { ReadOnlyNote } from '@shared/ui/read-only-note';

import { defaultHotDishesQueryOptions } from '../api/hot-dishes';

export type OrdersTab = 'history' | 'hot';

interface OrdersLayoutProps {
	pointId: string;
	tab: OrdersTab;
	children: ReactNode;
}

function ManualOrderAction({
	pointId,
	access,
}: {
	pointId: string;
	access: WriteAccess;
}) {
	if (access === 'forbidden') {
		return null;
	}

	return access === 'allowed' ? (
		<ButtonLink
			to='/points/$pointId/orders/new'
			params={{ pointId }}
			variant='outlined'
			startIcon={<Icon name='manualOrder' />}
		>
			Оформить вручную
		</ButtonLink>
	) : (
		<Button variant='outlined' disabled startIcon={<Icon name='manualOrder' />}>
			Оформить вручную
		</Button>
	);
}

/** Раздел «Заказы» точки: вкладки «История» и «Горячие предложения», ручное оформление. */
export function OrdersLayout({ pointId, tab, children }: OrdersLayoutProps) {
	const { profile, point } = usePoint(pointId);
	const mayRecommend = can(point, 'recommendations.view');
	const hot = useQuery({
		...defaultHotDishesQueryOptions(pointId),
		enabled: mayRecommend,
	});
	const access = writeAccess(profile, point, 'orders.accept');

	return (
		<PageContent>
			<PageHeader
				title='Заказы'
				meta={`${point.name} · входящие заказы, сначала новые`}
				actions={<ManualOrderAction pointId={pointId} access={access} />}
			/>
			{access === 'disabled' && (
				<Box sx={{ mb: 2.5 }}>
					<ReadOnlyNote>
						Точка отключена — заказы на ней не оформляются. Историю можно
						смотреть.
					</ReadOnlyNote>
				</Box>
			)}
			<SectionTabs value={tab} label='Разделы заказов'>
				<TabLink
					value='history'
					label='История'
					to='/points/$pointId/orders'
					params={{ pointId }}
				/>
				{mayRecommend && (
					<TabLink
						value='hot'
						label={
							<TabLabel text='Горячие предложения' count={hot.data?.total} />
						}
						to='/points/$pointId/orders/hot'
						params={{ pointId }}
					/>
				)}
			</SectionTabs>
			{children}
		</PageContent>
	);
}
