import { useQuery } from '@tanstack/react-query';

import { deliverySummaryQueryOptions } from '@entities/delivery';
import { menuQueryOptions } from '@entities/dish';
import { urgentExpiringHours } from '@entities/stock';
import { CardLink } from '@shared/ui/link';

import {
	expiringQueryOptions,
	hotQueryOptions,
	pendingQueryOptions,
	quarantineDeliveriesQueryOptions,
} from '../api/queries';
import { blockQuery } from '../model/block-query';
import {
	expiringTile,
	hotTile,
	pendingTile,
	quarantineTile,
	stopListTile,
} from '../model/tiles';
import { StatTile } from './stat-tile';

export function ExpiringTile({ pointId, now }: { pointId: string; now: Date }) {
	const expiring = useQuery(expiringQueryOptions(pointId));

	return (
		<StatTile
			icon='expiryWarning'
			label='Истекают за сутки'
			query={blockQuery(expiring)}
			tile={expiring.data && expiringTile(expiring.data, now)}
			link={(content) => (
				<CardLink
					to='/points/$pointId/stock/expiring'
					params={{ pointId }}
					search={{ within: urgentExpiringHours }}
				>
					{content}
				</CardLink>
			)}
		/>
	);
}

export function StopListTile({ pointId }: { pointId: string }) {
	const menu = useQuery(menuQueryOptions(pointId));

	return (
		<StatTile
			icon='stopList'
			label='Стоп-лист'
			query={blockQuery(menu)}
			tile={menu.data && stopListTile(menu.data)}
			link={(content) => (
				<CardLink to='/points/$pointId/stop-list' params={{ pointId }}>
					{content}
				</CardLink>
			)}
		/>
	);
}

export function HotTile({ pointId }: { pointId: string }) {
	const hot = useQuery(hotQueryOptions(pointId));

	return (
		<StatTile
			icon='hotDish'
			label='Горячие предложения'
			query={blockQuery(hot)}
			tile={hot.data && hotTile(hot.data.total)}
			link={(content) => (
				<CardLink to='/points/$pointId/orders/hot' params={{ pointId }}>
					{content}
				</CardLink>
			)}
		/>
	);
}

export function PendingTile({ pointId, now }: { pointId: string; now: Date }) {
	const pending = useQuery(pendingQueryOptions(pointId));

	return (
		<StatTile
			icon='deliveries'
			label='Ожидают приёмки'
			query={blockQuery(pending)}
			tile={pending.data && pendingTile(pending.data, now)}
			link={(content) => (
				<CardLink
					to='/points/$pointId/deliveries'
					params={{ pointId }}
					search={{ status: 'registered' }}
				>
					{content}
				</CardLink>
			)}
		/>
	);
}

export function QuarantineTile({ pointId }: { pointId: string }) {
	const summary = useQuery(deliverySummaryQueryOptions(pointId));
	const deliveries = useQuery(quarantineDeliveriesQueryOptions(pointId));

	return (
		<StatTile
			icon='quarantine'
			label='Открытый карантин'
			query={blockQuery(summary, deliveries)}
			tile={
				summary.data &&
				deliveries.data &&
				quarantineTile(summary.data, deliveries.data.total)
			}
			link={(content) => (
				<CardLink
					to='/points/$pointId/deliveries'
					params={{ pointId }}
					search={{ panel: 'quarantine' }}
				>
					{content}
				</CardLink>
			)}
		/>
	);
}
