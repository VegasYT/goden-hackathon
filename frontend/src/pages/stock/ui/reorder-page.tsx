import {
	Box,
	Button,
	Chip,
	Collapse,
	Paper,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Tooltip,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { ProductHistoryLink } from '@entities/product';
import { formatAmount } from '@shared/lib/quantity';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { EmptyValue } from '@shared/ui/empty-value';
import { InfoNote } from '@shared/ui/info-note';
import { DisclosureIcon, Icon } from '@shared/ui/icon';
import { TextLink } from '@shared/ui/link';
import { ListPagination, usePagedSearch } from '@shared/ui/list-pagination';
import { PanelHeader } from '@shared/ui/panel-header';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import {
	deliveryRecommendationsQueryOptions,
	supplyHours,
	type DeliveryRecommendation,
} from '../api/recommendations';
import { coverLabel } from '../model/reorder';
import { toRecommendationPage, type ReorderSearch } from '../model/search';
import { AvailableAmount } from './parts';

interface ReorderPageProps {
	pointId: string;
	search: ReorderSearch;
	onSearchChange: (patch: Partial<ReorderSearch>) => void;
}

/** Сколько блюд называть по имени; остальные — «и ещё N». */
const shownDishes = 2;

function Consumed({ item }: { item: DeliveryRecommendation }) {
	return item.consumed_quantity === 0 ? (
		<Box component='span' sx={{ color: 'text.secondary' }}>
			не было
		</Box>
	) : (
		<>{formatAmount(item.consumed_quantity, item.product.unit)}</>
	);
}

function Cover({ item }: { item: DeliveryRecommendation }) {
	const label = coverLabel(item);

	return label ? <Chip color='warning' label={label} /> : <EmptyValue />;
}

function Suggested({ item }: { item: DeliveryRecommendation }) {
	return (
		<Box component='span' sx={{ fontWeight: 600, color: 'text.strong' }}>
			{formatAmount(item.suggested_quantity, item.product.unit)}
		</Box>
	);
}

function Dishes({ item }: { item: DeliveryRecommendation }) {
	const names = item.needed_by.map((dish) => dish.name);
	const rest = names.slice(shownDishes);

	return (
		<>
			{names.slice(0, shownDishes).join(', ')}
			{rest.length > 0 && (
				<>
					{' '}
					<Tooltip title={rest.join(', ')}>
						<Box
							component='span'
							tabIndex={0}
							aria-label={`и ещё ${rest.length}: ${rest.join(', ')}`}
							sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}
						>
							и ещё {rest.length}
						</Box>
					</Tooltip>
				</>
			)}
		</>
	);
}

function ReorderTable({ items }: { items: DeliveryRecommendation[] }) {
	return (
		<Table aria-label='Рекомендации поставок' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Продукт</TableCell>
					<TableCell sx={{ width: 140 }}>Доступно</TableCell>
					<TableCell sx={{ width: 130 }}>Расход за 24 ч</TableCell>
					<TableCell sx={{ width: 130 }}>Хватит на</TableCell>
					<TableCell sx={{ width: 120 }}>Заказать</TableCell>
					<TableCell sx={{ width: 340 }}>Нужен для</TableCell>
				</TableRow>
			</TableHead>
			<TableBody>
				{items.map((item) => (
					<TableRow key={item.product.id}>
						<TableCell sx={{ fontWeight: 500 }}>
							<ProductHistoryLink product={item.product} inPlace />
						</TableCell>
						<TableCell>
							<AvailableAmount
								quantity={item.available_quantity}
								unit={item.product.unit}
							/>
						</TableCell>
						<TableCell>
							<Consumed item={item} />
						</TableCell>
						<TableCell>
							<Cover item={item} />
						</TableCell>
						<TableCell>
							<Suggested item={item} />
						</TableCell>
						<TableCell>
							<Dishes item={item} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function ReorderCards({ items }: { items: DeliveryRecommendation[] }) {
	return (
		<CardList label='Рекомендации поставок'>
			{items.map((item) => (
				<CardListItem key={item.product.id} sx={{ display: 'block' }}>
					<Stack direction='row' sx={{ gap: 1.5, alignItems: 'flex-start' }}>
						<Box sx={{ flex: 1, minWidth: 0 }}>
							<Typography variant='h3'>
								<ProductHistoryLink product={item.product} inPlace />
							</Typography>
							{item.needed_by.length > 0 && (
								<Typography variant='body2' sx={{ color: 'text.secondary' }}>
									для блюд: <Dishes item={item} />
								</Typography>
							)}
						</Box>
						<Typography sx={{ whiteSpace: 'nowrap' }}>
							<Box
								component='span'
								sx={{ color: 'text.secondary', fontSize: 13 }}
							>
								заказать{' '}
							</Box>
							<Box component='span' sx={{ fontSize: 20 }}>
								<Suggested item={item} />
							</Box>
						</Typography>
					</Stack>
					<Box
						component='dl'
						sx={{
							display: 'grid',
							gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
							gap: 1.5,
							m: 0,
							mt: 1.25,
							pt: 1.25,
							borderTop: '1px solid',
							borderColor: 'divider',
						}}
					>
						{(
							[
								[
									'Доступно',
									<AvailableAmount
										quantity={item.available_quantity}
										unit={item.product.unit}
									/>,
								],
								['Расход за 24 ч', <Consumed item={item} />],
								['Хватит на', <Cover item={item} />],
							] as const
						).map(([label, value]) => (
							<Box key={label}>
								<Typography
									component='dt'
									variant='body2'
									sx={{ color: 'text.secondary' }}
								>
									{label}
								</Typography>
								<Box component='dd' sx={{ m: 0, mt: 0.25 }}>
									{value}
								</Box>
							</Box>
						))}
					</Box>
				</CardListItem>
			))}
		</CardList>
	);
}

/** Вкладка «Пора заказать»: продукты, которых не хватит на сутки или уже не хватает блюдам. */
export function ReorderPage({
	pointId,
	search,
	onSearchChange,
}: ReorderPageProps) {
	const wide = useWideLayout();
	const explanationId = useId();
	const [explained, setExplained] = useState(false);
	const list = useQuery(
		deliveryRecommendationsQueryOptions(pointId, toRecommendationPage(search))
	);

	const { pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	const content = (data: NonNullable<typeof list.data>) => {
		if (data.items.length === 0) {
			return (
				<EmptyState
					icon='empty'
					title='Заказывать пока нечего'
					description='При нынешнем расходе запаса всех продуктов хватит больше чем на сутки'
				/>
			);
		}

		const rows = { items: data.items };

		return (
			<>
				{wide ? <ReorderTable {...rows} /> : <ReorderCards {...rows} />}
				<ListPagination total={data.total} {...pagination} />
			</>
		);
	};

	return (
		<>
			<InfoNote sx={{ mb: 1.5 }}>
				Этих продуктов не хватит на ближайшие сутки, или их уже не хватает
				блюдам. Пришедшую поставку зарегистрируйте в разделе{' '}
				<TextLink to='/points/$pointId/deliveries' params={{ pointId }}>
					Поставки
				</TextLink>
				.
			</InfoNote>
			<Paper sx={{ overflow: 'hidden' }}>
				<PanelHeader>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						Расход за {supplyHours.window_hours} часа · заказывать, когда запаса
						меньше чем на {supplyHours.cover_hours} часа · доводить запас до{' '}
						{supplyHours.target_hours} часов
					</Typography>
					<Button
						size='small'
						aria-expanded={explained}
						aria-controls={explanationId}
						startIcon={<Icon name='info' size='sm' />}
						endIcon={
							<DisclosureIcon
								open={explained}
								sx={{ fontSize: 16, color: 'inherit' }}
							/>
						}
						onClick={() => setExplained((open) => !open)}
					>
						Как считаем
					</Button>
				</PanelHeader>
				<Collapse in={explained} id={explanationId}>
					<Typography
						variant='body2'
						sx={{
							px: 2,
							py: 1.5,
							color: 'text.secondary',
							bgcolor: 'background.default',
							borderBottom: '1px solid',
							borderColor: 'divider',
						}}
					>
						«Заказать» — столько, чтобы при нынешнем расходе запаса хватило на{' '}
						{supplyHours.target_hours} часа. Расход считаем по заказам точки за
						последние {supplyHours.window_hours} часа, просроченные партии в
						доступное не входят. Если расхода не было, а продукта не хватает на
						порцию блюда, предлагаем запас на 10 порций.
					</Typography>
				</Collapse>
				<QueryState
					query={list}
					errorTitle='Не удалось загрузить рекомендации'
					loading={
						<SkeletonList
							label='Загружаем рекомендации'
							count={5}
							height={40}
							sx={{ p: 2 }}
						/>
					}
				>
					{content}
				</QueryState>
			</Paper>
		</>
	);
}
