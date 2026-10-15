import {
	Box,
	Button,
	ButtonBase,
	Chip,
	type ChipProps,
	Paper,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { Fragment } from 'react';

import { mayWrite, usePoint } from '@entities/profile';
import { actorName } from '@entities/actor';
import { formatDateTime, formatEventTime } from '@shared/lib/datetime';
import { useToggleSet } from '@shared/lib/toggle-set';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { EmptyValue } from '@shared/ui/empty-value';
import { DisclosureIcon } from '@shared/ui/icon';
import { ListPagination, usePagedSearch } from '@shared/ui/list-pagination';
import { PanelHeader } from '@shared/ui/panel-header';
import {
	SegmentedControl,
	type SegmentedOption,
} from '@shared/ui/segmented-control';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { coarsePointer, radius, useWideLayout } from '@shared/ui/theme';

import {
	orderListQueryOptions,
	type Order,
	type OrderPage,
} from '../api/orders';
import {
	cancelledBy,
	cancelledText,
	dishesText,
	placedTime,
	shortagesText,
} from '../model/order';
import {
	toOrderListParams,
	type HistorySearch,
	type OrderStatusFilter,
} from '../model/search';
import { OrderActions, OrderActor, OrderItems } from './order-cancel';
import { ShortageTable } from './shortage-table';
import { useOrderCancel } from './use-order-cancel';

interface HistoryPageProps {
	pointId: string;
	search: HistorySearch;
	onSearchChange: (patch: Partial<HistorySearch>) => void;
}

const statusOptions: SegmentedOption<OrderStatusFilter>[] = [
	{ value: 'all', label: 'Все' },
	{ value: 'accepted', label: 'Принятые' },
	{ value: 'rejected', label: 'Отклонённые' },
	{ value: 'cancelled', label: 'Отменённые' },
];

const emptyFiltered: Record<
	Exclude<OrderStatusFilter, 'all'>,
	{ title: string; description: string }
> = {
	accepted: {
		title: 'Принятых заказов нет',
		description: 'Пока ни один заказ не был принят',
	},
	rejected: {
		title: 'Отклонённых заказов нет',
		description: 'Всем заказам точки хватило продуктов',
	},
	cancelled: {
		title: 'Отменённых заказов нет',
		description: 'Ни один принятый заказ точки не отменили',
	},
};

const detailsId = (order: Order) => `order-${order.id}`;

const statusChips: Record<Order['status'], ChipProps> = {
	accepted: { color: 'success', label: 'принят' },
	rejected: { color: 'error', label: 'отклонён' },
	cancelled: { label: 'отменён' },
};

function StatusChip({ order }: { order: Order }) {
	return <Chip {...statusChips[order.status]} />;
}

const strong = { fontWeight: 500, color: 'text.strong' } as const;

/** Подробности статуса: у отклонённого — чего не хватило, у отменённого — кто, когда и с каким комментарием. */
function Details({ order }: { order: Order }) {
	if (order.status === 'rejected') {
		return (
			<>
				Не хватило:{' '}
				<Box component='span' sx={strong}>
					{shortagesText(order)}
				</Box>
			</>
		);
	}

	if (order.status === 'cancelled') {
		return (
			<>
				Отменил{' '}
				<Box component='span' sx={strong}>
					{cancelledBy(order)}
				</Box>
				{order.cancelled_at && `, ${formatEventTime(order.cancelled_at)}`}
				{order.cancel_comment && ` · «${order.cancel_comment}»`}
			</>
		);
	}

	return <EmptyValue />;
}

/** Время заказа кнопкой раскрытия: строка раскрывается в состав и подробности статуса. */
function OrderToggle({
	order,
	open,
	onToggle,
}: {
	order: Order;
	open: boolean;
	onToggle: () => void;
}) {
	return (
		<ButtonBase
			aria-expanded={open}
			aria-controls={open ? detailsId(order) : undefined}
			aria-label={`Заказ ${placedTime(order)}`}
			onClick={onToggle}
			sx={(theme) => ({
				display: 'flex',
				alignItems: 'center',
				gap: 0.75,
				borderRadius: radius(theme, 'sm'),
				[coarsePointer]: { minHeight: 44 },
			})}
		>
			<DisclosureIcon open={open} />
			<Typography sx={{ color: 'text.strong' }}>{placedTime(order)}</Typography>
		</ButtonBase>
	);
}

function OrderDetails({ order }: { order: Order }) {
	return (
		<Box
			id={detailsId(order)}
			sx={{
				display: 'grid',
				gridTemplateColumns: { xs: '1fr', lg: '280px minmax(0, 1fr)' },
				gap: { xs: 2, lg: 4 },
			}}
		>
			<Box>
				<Typography variant='body2' sx={{ color: 'text.secondary', mb: 0.5 }}>
					Состав
				</Typography>
				<OrderItems items={order.items} />
			</Box>
			{order.status === 'rejected' && (
				<Box>
					<Typography variant='body2' sx={{ color: 'text.secondary', mb: 0.5 }}>
						Чего не хватило · заказ отклонён целиком, склад не изменился
					</Typography>
					<ShortageTable shortages={order.shortages} />
				</Box>
			)}
			{order.status === 'cancelled' && <CancelledDetails order={order} />}
		</Box>
	);
}

function CancelledDetails({ order }: { order: Order }) {
	const at = (value?: string) => (value ? `, ${formatDateTime(value)}` : '');
	const rows = [
		['Оформил', actorName(order.placed_by) + at(order.placed_at)],
		['Отменил', cancelledBy(order) + at(order.cancelled_at)],
		...(order.cancel_comment
			? [['Комментарий', `«${order.cancel_comment}»`]]
			: []),
	];

	return (
		<Box>
			<Typography variant='body2' sx={{ color: 'text.secondary', mb: 0.5 }}>
				Заказ отменён · продукты вернулись на склад
			</Typography>
			{rows.map(([label, value]) => (
				<Stack key={label} direction='row' sx={{ gap: 1.5, py: 0.5 }}>
					<Typography
						sx={{ width: 96, flexShrink: 0, color: 'text.secondary' }}
					>
						{label}
					</Typography>
					<Typography sx={{ color: 'text.strong' }}>{value}</Typography>
				</Stack>
			))}
		</Box>
	);
}

interface OrderViewProps {
	orders: Order[];
	expanded: ReadonlySet<string>;
	onToggle: (id: string) => void;
	/** Отмена заказа; без неё у заказов нет меню действий. */
	onCancel?: (order: Order) => void;
}

function Actions({
	order,
	onCancel,
}: Pick<OrderViewProps, 'onCancel'> & { order: Order }) {
	return onCancel && order.status === 'accepted' ? (
		<OrderActions order={order} onCancel={onCancel} />
	) : null;
}

function OrderTable({ orders, expanded, onToggle, onCancel }: OrderViewProps) {
	return (
		<Table aria-label='История заказов' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell sx={{ width: 170 }}>Оформлен</TableCell>
					<TableCell sx={{ width: 160 }}>Оформил</TableCell>
					<TableCell>Блюда</TableCell>
					<TableCell sx={{ width: 120 }}>Статус</TableCell>
					<TableCell sx={{ width: 300 }}>Подробности</TableCell>
					{onCancel && <TableCell sx={{ width: 56 }} />}
				</TableRow>
			</TableHead>
			<TableBody>
				{orders.map((order) => {
					const open = expanded.has(order.id);

					return (
						<Fragment key={order.id}>
							<TableRow sx={{ bgcolor: open ? 'field' : undefined }}>
								<TableCell>
									<OrderToggle
										order={order}
										open={open}
										onToggle={() => onToggle(order.id)}
									/>
								</TableCell>
								<TableCell>
									<OrderActor actor={order.placed_by} />
								</TableCell>
								<TableCell sx={{ color: 'text.strong' }}>
									<Typography noWrap title={dishesText(order.items)}>
										{dishesText(order.items)}
									</Typography>
								</TableCell>
								<TableCell>
									<StatusChip order={order} />
								</TableCell>
								<TableCell>
									<Typography
										noWrap
										title={
											order.status === 'cancelled'
												? cancelledText(order)
												: undefined
										}
									>
										<Details order={order} />
									</Typography>
								</TableCell>
								{onCancel && (
									<TableCell sx={{ py: 0.5, px: 1 }}>
										<Actions order={order} onCancel={onCancel} />
									</TableCell>
								)}
							</TableRow>
							{open && (
								<TableRow sx={{ bgcolor: 'field' }}>
									<TableCell colSpan={onCancel ? 6 : 5} sx={{ pt: 0, pl: 5 }}>
										<OrderDetails order={order} />
									</TableCell>
								</TableRow>
							)}
						</Fragment>
					);
				})}
			</TableBody>
		</Table>
	);
}

function OrderCards({ orders, expanded, onToggle, onCancel }: OrderViewProps) {
	return (
		<CardList label='История заказов'>
			{orders.map((order) => {
				const open = expanded.has(order.id);

				return (
					<CardListItem key={order.id} sx={{ display: 'block' }}>
						<Stack direction='row' sx={{ gap: 1, alignItems: 'center' }}>
							<OrderToggle
								order={order}
								open={open}
								onToggle={() => onToggle(order.id)}
							/>
							<OrderActor actor={order.placed_by} />
							<Box sx={{ flexGrow: 1 }} />
							<StatusChip order={order} />
							<Actions order={order} onCancel={onCancel} />
						</Stack>
						<Box sx={{ pl: 3.5 }}>
							<Typography sx={{ color: 'text.strong' }}>
								{dishesText(order.items)}
							</Typography>
							{order.status !== 'accepted' && (
								<Typography variant='body2' sx={{ color: 'text.secondary' }}>
									<Details order={order} />
								</Typography>
							)}
							{open && (
								<Box sx={{ mt: 1.5 }}>
									<OrderDetails order={order} />
								</Box>
							)}
						</Box>
					</CardListItem>
				);
			})}
		</CardList>
	);
}

/** Вкладка «История»: заказы точки от новых к старым, фильтр и страницы считает сервер. */
export function HistoryPage({
	pointId,
	search,
	onSearchChange,
}: HistoryPageProps) {
	const wide = useWideLayout();
	const [expanded, toggle] = useToggleSet();
	const { profile, point } = usePoint(pointId);
	const cancel = useOrderCancel(pointId);
	const onCancel = mayWrite(profile, point, 'orders.cancel')
		? cancel.open
		: undefined;
	const list = useQuery(
		orderListQueryOptions(pointId, toOrderListParams(search))
	);

	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	const empty = () =>
		search.status === 'all' ? (
			<EmptyState
				icon='orders'
				title='Заказов пока нет'
				description='Здесь появятся заказы, которые приходят на точку: принятые, отклонённые и отменённые'
			/>
		) : (
			<EmptyState
				icon='done'
				{...emptyFiltered[search.status]}
				action={
					<Button variant='outlined' onClick={() => filter({ status: 'all' })}>
						Показать все заказы
					</Button>
				}
			/>
		);

	const content = (data: OrderPage) => {
		if (data.items.length === 0) {
			return empty();
		}

		const view = {
			orders: data.items,
			expanded,
			onToggle: toggle,
			onCancel,
		};

		return (
			<>
				{wide ? <OrderTable {...view} /> : <OrderCards {...view} />}
				<ListPagination total={data.total} {...pagination} />
			</>
		);
	};

	return (
		<Paper sx={{ overflow: 'hidden' }}>
			<PanelHeader>
				<SegmentedControl
					label='Статус заказа'
					options={statusOptions}
					value={search.status}
					onChange={(status) => filter({ status })}
				/>
			</PanelHeader>
			<QueryState
				query={list}
				loading={
					<SkeletonList label='Загружаем заказы' height={40} sx={{ p: 2 }} />
				}
				errorTitle='Не удалось загрузить заказы'
			>
				{content}
			</QueryState>
			{cancel.elements}
		</Paper>
	);
}
