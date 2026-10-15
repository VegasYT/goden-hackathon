import {
	Alert,
	Box,
	Button,
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
import { useState } from 'react';

import { ProductHistoryLink } from '@entities/product';
import { usePoint, writeAccess, type WriteAccess } from '@entities/profile';
import {
	BatchExpiryLabel,
	expiringBatchesQueryOptions,
	type ExpiringBatch,
} from '@entities/stock';
import {
	formatDateTime,
	hourWindowLabels,
	hourWindows,
	type HourWindow,
} from '@shared/lib/datetime';
import { isExpired, useNow } from '@shared/lib/expiry';
import { countOf } from '@shared/lib/plural';
import { formatAmount } from '@shared/lib/quantity';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { Icon } from '@shared/ui/icon';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { ListPagination, usePagedSearch } from '@shared/ui/list-pagination';
import { PanelHeader } from '@shared/ui/panel-header';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import { stockSummaryQueryOptions } from '../api/stock';
import { toExpiringParams, type ExpiringSearch } from '../model/search';
import {
	BatchActionDialogs,
	type OpenBatchAction,
} from './batch-action-dialogs';
import { DeliveryLink, InventoryReadOnlyNote, WriteOffButton } from './parts';

interface ExpiringPageProps {
	pointId: string;
	search: ExpiringSearch;
	onSearchChange: (patch: Partial<ExpiringSearch>) => void;
}

const longestPeriod = hourWindows.at(-1)!;

interface ListProps {
	pointId: string;
	items: ExpiringBatch[];
	now: Date;
	access: WriteAccess;
	onWriteOff: (item: ExpiringBatch) => void;
}

function ExpiringWriteOff({
	item,
	list,
	touch = false,
}: {
	item: ExpiringBatch;
	list: ListProps;
	touch?: boolean;
}) {
	if (list.access !== 'allowed') {
		return null;
	}

	return (
		<WriteOffButton
			expired={isExpired(item.batch.expires_at, list.now)}
			touch={touch}
			label={`Списать партию № ${item.batch.delivery_number}: ${item.product.name}`}
			onClick={() => list.onWriteOff(item)}
			sx={touch ? { flexShrink: 0 } : undefined}
		/>
	);
}

function ExpiringTable(list: ListProps) {
	return (
		<Table aria-label='Истекающие партии' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Продукт</TableCell>
					<TableCell sx={{ width: 150 }}>Партия</TableCell>
					<TableCell sx={{ width: 110 }}>Остаток</TableCell>
					<TableCell sx={{ width: 130 }}>Годна до</TableCell>
					<TableCell sx={{ width: 200 }}>Статус</TableCell>
					<TableCell sx={{ width: 130 }} />
				</TableRow>
			</TableHead>
			<TableBody>
				{list.items.map((item) => (
					<TableRow key={item.batch.id} sx={{ verticalAlign: 'top' }}>
						<TableCell sx={{ fontWeight: 500 }}>
							<ProductHistoryLink product={item.product} inPlace />
							<Typography
								variant='caption'
								component='div'
								sx={{ color: 'text.secondary' }}
							>
								{item.product.category}
							</Typography>
						</TableCell>
						<TableCell>
							из поставки{' '}
							<DeliveryLink pointId={list.pointId} batch={item.batch} />
						</TableCell>
						<TableCell sx={{ fontWeight: 500 }}>
							{formatAmount(item.batch.quantity, item.product.unit)}
						</TableCell>
						<TableCell>
							{formatDateTime(item.batch.expires_at, list.now)}
						</TableCell>
						<TableCell>
							<BatchExpiryLabel
								expiresAt={item.batch.expires_at}
								now={list.now}
							/>
						</TableCell>
						<TableCell align='right'>
							<ExpiringWriteOff item={item} list={list} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function ExpiringCards(list: ListProps) {
	return (
		<CardList label='Истекающие партии'>
			{list.items.map((item) => (
				<CardListItem key={item.batch.id} sx={{ flexWrap: 'wrap' }}>
					<Box sx={{ flex: '1 1 220px', minWidth: 0 }}>
						<Typography variant='h3'>
							<ProductHistoryLink product={item.product} inPlace />
						</Typography>
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							из поставки{' '}
							<DeliveryLink pointId={list.pointId} batch={item.batch} /> ·
							остаток{' '}
							<Box
								component='span'
								sx={{ fontWeight: 600, color: 'text.strong' }}
							>
								{formatAmount(item.batch.quantity, item.product.unit)}
							</Box>
						</Typography>
					</Box>
					<Box sx={{ flex: '0 1 220px' }}>
						<BatchExpiryLabel
							expiresAt={item.batch.expires_at}
							now={list.now}
							inline
						/>
					</Box>
					<ExpiringWriteOff item={item} list={list} touch />
				</CardListItem>
			))}
		</CardList>
	);
}

/** Вкладка «Сроки»: партии точки, срок которых истекает за выбранный период или уже истёк. */
export function ExpiringPage({
	pointId,
	search,
	onSearchChange,
}: ExpiringPageProps) {
	const wide = useWideLayout();
	const now = useNow();
	const { profile, point } = usePoint(pointId);
	const access = writeAccess(profile, point, 'inventory.edit');
	const list = useQuery(
		expiringBatchesQueryOptions(pointId, toExpiringParams(search))
	);
	// Просрочку считает сводка склада: на текущей странице видны не все просроченные партии.
	const summary = useQuery(stockSummaryQueryOptions(pointId, {}));
	const expiredProducts = summary.data?.expired ?? 0;
	const [action, setAction] = useState<OpenBatchAction | null>(null);

	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	const content = (data: NonNullable<typeof list.data>) => {
		if (data.items.length === 0) {
			return (
				<EmptyState
					icon='expiryWarning'
					title={`За ${hourWindowLabels[search.within]} ничего не истекает`}
					description='Партий с таким сроком на складе нет'
					action={
						search.within !== longestPeriod && (
							<Button
								variant='outlined'
								onClick={() => filter({ within: longestPeriod })}
							>
								Показать за {hourWindowLabels[longestPeriod]}
							</Button>
						)
					}
				/>
			);
		}

		const rows: ListProps = {
			pointId,
			items: data.items,
			now,
			access,
			onWriteOff: ({ product, batch }) =>
				setAction({ kind: 'writeOff', product, batch }),
		};

		return (
			<>
				{wide ? <ExpiringTable {...rows} /> : <ExpiringCards {...rows} />}
				{access !== 'allowed' && (
					<Box sx={{ px: 2, pb: 2, pt: wide ? 2 : 0 }}>
						<InventoryReadOnlyNote access={access} />
					</Box>
				)}
				<ListPagination total={data.total} {...pagination} />
			</>
		);
	};

	return (
		<>
			{expiredProducts > 0 && (
				<Alert severity='error' icon={<Icon name='expired' />} sx={{ mb: 1.5 }}>
					<b>
						Просрочка у{' '}
						{countOf(expiredProducts, 'продукта', 'продуктов', 'продуктов')}.
					</b>{' '}
					Сама она не спишется: проверьте и спишите партии. В заказы и меню
					просрочка не идёт.
				</Alert>
			)}
			<Paper sx={{ overflow: 'hidden' }}>
				<PanelHeader>
					<Stack direction='row' sx={{ gap: 1.5, alignItems: 'center' }}>
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							Истекают за
						</Typography>
						<SegmentedControl
							label='Истекают за'
							options={hourWindows.map((value) => ({
								value: String(value),
								label: hourWindowLabels[value],
							}))}
							value={String(search.within)}
							onChange={(value) =>
								filter({ within: Number(value) as HourWindow })
							}
						/>
					</Stack>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						сначала ближайший срок
					</Typography>
				</PanelHeader>
				<QueryState
					query={list}
					errorTitle='Не удалось загрузить партии'
					loading={
						<SkeletonList
							label='Загружаем партии'
							count={5}
							height={40}
							sx={{ p: 2 }}
						/>
					}
				>
					{content}
				</QueryState>
			</Paper>
			<BatchActionDialogs
				pointId={pointId}
				now={now}
				action={action}
				onClose={() => setAction(null)}
			/>
		</>
	);
}
