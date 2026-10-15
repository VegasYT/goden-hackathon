import {
	Box,
	Chip,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useNavigate } from '@tanstack/react-router';

import { actorName } from '@entities/actor';
import {
	DeliveryStatusChip,
	expiredItems,
	itemOutcomes,
	positionsCount,
	type Delivery,
	type DeliverySort,
} from '@entities/delivery';
import { formatDateTime, formatLongDateTime } from '@shared/lib/datetime';
import { plural } from '@shared/lib/plural';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyValue } from '@shared/ui/empty-value';
import { CardLink, TextLink } from '@shared/ui/link';
import { SortableHeaderCell } from '@shared/ui/sortable-header-cell';

interface RowsProps {
	pointId: string;
	deliveries: Delivery[];
	now: Date;
}

/** Сроки ожидающей поставки: сколько позиций уже просрочено. */
function Terms({ delivery, now }: { delivery: Delivery; now: Date }) {
	if (delivery.status === 'approved') {
		return <EmptyValue />;
	}

	const expired = expiredItems(delivery, now);

	return expired > 0 ? (
		<Chip
			color='error'
			label={`${expired} ${plural(expired, 'просрочена', 'просрочены', 'просрочены')}`}
		/>
	) : (
		<Typography
			variant='body2'
			component='span'
			sx={{ color: 'text.secondary' }}
		>
			в порядке
		</Typography>
	);
}

function outcomeText(delivery: Delivery): string {
	const { toStock, rejected, quarantined } = itemOutcomes(delivery.items);
	const parts = [`${toStock} на склад`];

	if (rejected > 0) {
		parts.push(`${rejected} откл.`);
	}

	if (quarantined > 0) {
		parts.push(`${quarantined} в кар.`);
	}

	return parts.join(' · ');
}

function Quarantine({ delivery }: { delivery: Delivery }) {
	const open = itemOutcomes(delivery.items).quarantined;

	return open > 0 ? (
		<Chip color='warning' label={`ждёт решения · ${open}`} />
	) : (
		<EmptyValue />
	);
}

function Approval({ delivery }: { delivery: Delivery }) {
	if (!delivery.approved_by || !delivery.approved_at) {
		return <EmptyValue />;
	}

	return (
		<>
			<Typography variant='body2' component='div' sx={{ color: 'text.strong' }}>
				{actorName(delivery.approved_by)}
			</Typography>
			<Typography
				variant='caption'
				component='div'
				sx={{ color: 'text.secondary' }}
			>
				{formatDateTime(delivery.approved_at)}
			</Typography>
		</>
	);
}

interface DeliveryTableProps extends RowsProps {
	sort: DeliverySort;
	onSortChange: (sort: DeliverySort) => void;
}

/** Таблица поставок на широком экране; строка ведёт на карточку поставки. */
export function DeliveryTable({
	pointId,
	deliveries,
	now,
	sort,
	onSortChange,
}: DeliveryTableProps) {
	const navigate = useNavigate();
	const ascending = sort === 'delivered_at';

	return (
		<Table aria-label='Список поставок' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Поставка</TableCell>
					<SortableHeaderCell
						sx={{ width: 150 }}
						direction={ascending ? 'asc' : 'desc'}
						onSort={() =>
							onSortChange(ascending ? '-delivered_at' : 'delivered_at')
						}
					>
						Поступила
					</SortableHeaderCell>
					<TableCell sx={{ width: 140 }}>Сроки</TableCell>
					<TableCell sx={{ width: 160 }}>Статус</TableCell>
					<TableCell sx={{ width: 180 }}>Приёмка</TableCell>
					<TableCell sx={{ width: 200 }}>Итог</TableCell>
					<TableCell sx={{ width: 170 }}>Карантин</TableCell>
				</TableRow>
			</TableHead>
			<TableBody>
				{deliveries.map((delivery) => (
					<TableRow
						key={delivery.id}
						hover
						onClick={() =>
							void navigate({
								to: '/points/$pointId/deliveries/$deliveryId',
								params: { pointId, deliveryId: delivery.id },
							})
						}
						sx={{ cursor: 'pointer' }}
					>
						<TableCell>
							<TextLink
								to='/points/$pointId/deliveries/$deliveryId'
								params={{ pointId, deliveryId: delivery.id }}
								underline='hover'
								onClick={(event) => event.stopPropagation()}
								sx={{ fontWeight: 500, color: 'text.strong' }}
							>
								№ {delivery.number}
							</TextLink>
							<Typography
								variant='caption'
								component='div'
								sx={{ color: 'text.secondary' }}
							>
								{positionsCount(delivery.items.length)}
							</Typography>
						</TableCell>
						<TableCell>{formatDateTime(delivery.delivered_at, now)}</TableCell>
						<TableCell>
							<Terms delivery={delivery} now={now} />
						</TableCell>
						<TableCell>
							<DeliveryStatusChip status={delivery.status} />
						</TableCell>
						<TableCell>
							<Approval delivery={delivery} />
						</TableCell>
						<TableCell>
							{delivery.status === 'approved' ? (
								outcomeText(delivery)
							) : (
								<EmptyValue />
							)}
						</TableCell>
						<TableCell>
							<Quarantine delivery={delivery} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

/** Поставки карточками на узком экране. */
export function DeliveryCards({ pointId, deliveries, now }: RowsProps) {
	return (
		<CardList label='Список поставок'>
			{deliveries.map((delivery) => (
				<CardListItem key={delivery.id} sx={{ display: 'block', p: 0 }}>
					<CardLink
						to='/points/$pointId/deliveries/$deliveryId'
						params={{ pointId, deliveryId: delivery.id }}
						sx={{ p: 2, display: 'grid', gap: 0.75, justifyContent: 'stretch' }}
					>
						<Stack direction='row' sx={{ alignItems: 'center', gap: 1.5 }}>
							<Typography variant='h3' component='span'>
								№ {delivery.number}
							</Typography>
							<DeliveryStatusChip status={delivery.status} />
							<Box sx={{ flex: 1 }} />
							<Quarantine delivery={delivery} />
						</Stack>
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							{formatLongDateTime(delivery.delivered_at)} ·{' '}
							{positionsCount(delivery.items.length)}
						</Typography>
						<Stack
							direction='row'
							sx={{ alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}
						>
							{delivery.status === 'approved' ? (
								<>
									<Typography variant='body2'>
										{delivery.approved_by && actorName(delivery.approved_by)}
										{delivery.approved_at &&
											` · ${formatDateTime(delivery.approved_at, now)}`}
									</Typography>
									<Typography variant='body2' sx={{ color: 'text.strong' }}>
										{outcomeText(delivery)}
									</Typography>
								</>
							) : (
								<Terms delivery={delivery} now={now} />
							)}
						</Stack>
					</CardLink>
				</CardListItem>
			))}
		</CardList>
	);
}
