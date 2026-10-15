import { Button, Paper, Stack } from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import {
	deliveryListQueryOptions,
	deliverySummaryQueryOptions,
	positionsCount,
	type DeliverySummary,
} from '@entities/delivery';
import { mayWrite, usePoint } from '@entities/profile';
import { useNow } from '@shared/lib/expiry';
import { countOf } from '@shared/lib/plural';
import { CountBadge } from '@shared/ui/count-badge';
import { EmptyState, NoResultsState, QueryState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { ListPagination, usePagedSearch } from '@shared/ui/list-pagination';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { PanelHeader } from '@shared/ui/panel-header';
import { SearchField } from '@shared/ui/search-field';
import { SkeletonList } from '@shared/ui/skeleton-list';
import {
	SegmentedControl,
	type SegmentedOption,
} from '@shared/ui/segmented-control';
import { useWideLayout } from '@shared/ui/theme';

import {
	toListParams,
	type DeliveryListSearch,
	type DeliveryStatusFilter,
} from '../model/search';
import { DeliveryCards, DeliveryTable } from './delivery-rows';
import { QuarantinePanel } from './quarantine-panel';

interface DeliveriesPageProps {
	pointId: string;
	search: DeliveryListSearch;
	/** Меняет состояние списка в адресе. */
	onSearchChange: (patch: Partial<DeliveryListSearch>) => void;
}

function summaryText(summary: DeliverySummary): string {
	if (summary.registered === 0 && summary.open_quarantine_items === 0) {
		return 'Ничего не ждёт приёмки';
	}

	const parts = [
		`${countOf(summary.registered, 'ждёт', 'ждут', 'ждут')} приёмки`,
	];

	if (summary.open_quarantine_items > 0) {
		parts.push(
			`${positionsCount(summary.open_quarantine_items)} в открытом карантине`
		);
	}

	return parts.join(' · ');
}

/** Список поставок точки: статус, поиск по номеру, сортировка и страницы хранятся в адресе. */
export function DeliveriesPage({
	pointId,
	search,
	onSearchChange,
}: DeliveriesPageProps) {
	const wide = useWideLayout();
	const now = useNow();
	const summary = useQuery(deliverySummaryQueryOptions(pointId));
	const list = useQuery(
		deliveryListQueryOptions(pointId, toListParams(search))
	);
	const searching = search.number.trim() !== '';
	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);
	const { profile, point } = usePoint(pointId);
	const mayRegister = mayWrite(profile, point, 'deliveries.register');

	const statusOptions: SegmentedOption<DeliveryStatusFilter>[] = [
		{ value: 'all', label: 'Все', count: summary.data?.total },
		{
			value: 'registered',
			label: 'Ожидают приёмки',
			count: summary.data?.registered,
		},
		{ value: 'approved', label: 'Приняты', count: summary.data?.approved },
	];

	const empty = () => {
		if (searching) {
			return (
				<NoResultsState
					title={`Нет поставок с номером на «${search.number.trim()}»`}
					description='Поиск идёт по началу номера поставки'
					onReset={() => filter({ number: '' })}
				/>
			);
		}

		if (search.status === 'registered') {
			return (
				<EmptyState
					icon='done'
					title='Все поставки приняты'
					description='Новые поставки появятся здесь, когда их зарегистрируют'
					action={
						<Button
							variant='outlined'
							onClick={() => filter({ status: 'all' })}
						>
							Показать все поставки
						</Button>
					}
				/>
			);
		}

		return (
			<EmptyState
				title={
					search.status === 'approved'
						? 'Принятых поставок пока нет'
						: 'Поставок пока нет'
				}
				description='Поставки появятся здесь, когда их зарегистрируют'
			/>
		);
	};

	return (
		<PageContent>
			<PageHeader
				title='Поставки'
				meta={summary.data && summaryText(summary.data)}
				actions={
					<Stack direction='row' sx={{ gap: 1, flexWrap: 'wrap' }}>
						<Button
							variant='outlined'
							startIcon={<Icon name='quarantine' />}
							aria-expanded={search.panel === 'quarantine'}
							onClick={() => onSearchChange({ panel: 'quarantine' })}
						>
							Открытый карантин
							{summary.data && summary.data.open_quarantine_items > 0 && (
								<CountBadge
									count={summary.data.open_quarantine_items}
									sx={{ ml: 1 }}
								/>
							)}
						</Button>
						{mayRegister && (
							<ButtonLink
								to='/points/$pointId/deliveries/new'
								params={{ pointId }}
								variant='contained'
								startIcon={<Icon name='add' />}
							>
								Новая поставка
							</ButtonLink>
						)}
					</Stack>
				}
			/>
			<Paper sx={{ mt: 2.5, overflow: 'hidden' }}>
				<PanelHeader>
					<SegmentedControl
						label='Статус поставок'
						options={statusOptions}
						value={search.status}
						onChange={(status) => filter({ status })}
					/>
					<SearchField
						placeholder='Номер поставки'
						label='Поиск по номеру поставки'
						value={search.number}
						onChange={(number) => filter({ number })}
					/>
				</PanelHeader>
				<QueryState
					query={list}
					errorTitle='Не удалось загрузить поставки'
					loading={
						<SkeletonList
							label='Загружаем поставки'
							height={40}
							sx={{ p: 2 }}
						/>
					}
				>
					{(data) =>
						data.items.length === 0 ? (
							empty()
						) : (
							<>
								{wide ? (
									<DeliveryTable
										pointId={pointId}
										deliveries={data.items}
										now={now}
										sort={search.sort}
										onSortChange={(sort) => filter({ sort })}
									/>
								) : (
									<DeliveryCards
										pointId={pointId}
										deliveries={data.items}
										now={now}
									/>
								)}
								<ListPagination total={data.total} {...pagination} />
							</>
						)
					}
				</QueryState>
			</Paper>
			<QuarantinePanel
				pointId={pointId}
				open={search.panel === 'quarantine'}
				now={now}
				onClose={() => onSearchChange({ panel: undefined })}
			/>
		</PageContent>
	);
}
