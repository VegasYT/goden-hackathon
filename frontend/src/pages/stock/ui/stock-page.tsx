import { Paper } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { usePoint, writeAccess } from '@entities/profile';
import { useNow } from '@shared/lib/expiry';
import { useToggleSet } from '@shared/lib/toggle-set';
import { EmptyState, NoResultsState, QueryState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { ListPagination, usePagedSearch } from '@shared/ui/list-pagination';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import {
	stockListQueryOptions,
	stockSummaryQueryOptions,
	type Batch,
	type StockLevel,
} from '../api/stock';
import {
	isFiltered,
	toFilterParams,
	toStockParams,
	type StockSearch,
} from '../model/search';
import {
	BatchActionDialogs,
	type OpenBatchAction,
} from './batch-action-dialogs';
import type { BatchAction } from './batch-list';
import { StockFilters } from './stock-filters';
import { StockCards, StockTable } from './stock-rows';
import { useHistoryPanel } from './use-history-panel';

interface StockPageProps {
	pointId: string;
	search: StockSearch;
	onSearchChange: (patch: Partial<StockSearch>) => void;
}

/** Пустой результат по фильтрам: что именно искали. */
function notFoundText(search: StockSearch): string {
	const query = search.q.trim();

	if (search.category && query) {
		return `В категории «${search.category}» нет продуктов по запросу «${query}»`;
	}

	if (query) {
		return `Нет продуктов, название которых начинается с «${query}»`;
	}

	return search.category
		? `В категории «${search.category}» нет подходящих продуктов`
		: 'Нет продуктов в выбранном состоянии';
}

/** Вкладка «Остатки»: продукты с партиями, списание и исправление остатка. */
export function StockPage({ pointId, search, onSearchChange }: StockPageProps) {
	const openHistory = useHistoryPanel();
	const wide = useWideLayout();
	const now = useNow();
	const { profile, point } = usePoint(pointId);
	const access = writeAccess(profile, point, 'inventory.edit');
	const list = useQuery(stockListQueryOptions(pointId, toStockParams(search)));
	const summary = useQuery(
		stockSummaryQueryOptions(pointId, toFilterParams(search))
	);
	const companyId = point.company_id;
	const [expanded, toggle] = useToggleSet();
	const [action, setAction] = useState<OpenBatchAction | null>(null);

	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	const emptyWarehouse =
		!isFiltered(search) &&
		summary.data !== undefined &&
		summary.data.in_stock === 0 &&
		summary.data.expired === 0;

	const content = (data: NonNullable<typeof list.data>) => {
		if (emptyWarehouse) {
			return (
				<EmptyState
					icon='stock'
					title='На складе пока нет партий'
					description='Они появятся после приёмки поставки'
					action={
						<ButtonLink
							to='/points/$pointId/deliveries'
							params={{ pointId }}
							variant='outlined'
							startIcon={<Icon name='deliveries' />}
						>
							К поставкам
						</ButtonLink>
					}
				/>
			);
		}

		if (data.items.length === 0) {
			return (
				<NoResultsState
					description={notFoundText(search)}
					resetLabel='Сбросить фильтры'
					onReset={() => filter({ q: '', category: '', status: 'all' })}
				/>
			);
		}

		const rows = {
			pointId,
			items: data.items,
			now,
			access,
			expanded,
			onToggle: toggle,
			onHistory: openHistory,
			onAction: (product: StockLevel, kind: BatchAction, batch: Batch) =>
				setAction({ kind, product: product.product, batch }),
		};

		return (
			<>
				{wide ? (
					<StockTable
						{...rows}
						sort={search.sort}
						onSortChange={(sort) => filter({ sort })}
					/>
				) : (
					<StockCards {...rows} />
				)}
				<ListPagination total={data.total} {...pagination} />
			</>
		);
	};

	return (
		<>
			<Paper sx={{ overflow: 'hidden' }}>
				<StockFilters
					search={search}
					summary={summary.data}
					companyId={companyId}
					wide={wide}
					onChange={filter}
				/>
				<QueryState
					query={list}
					errorTitle='Не удалось загрузить остатки'
					loading={
						<SkeletonList
							label='Загружаем остатки'
							count={6}
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
