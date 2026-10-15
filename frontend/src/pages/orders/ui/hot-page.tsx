import {
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

import {
	hotDishesQueryOptions,
	PinnedChip,
	portionsText,
	suggestedPortionsText,
	type HotDish,
} from '@entities/dish';
import { ProductHistoryLink } from '@entities/product';
import { can, mayWrite, usePoint } from '@entities/profile';
import {
	hourWindowLabels,
	hourWindows,
	type HourWindow,
} from '@shared/lib/datetime';
import { useNow } from '@shared/lib/expiry';
import { formatAmount } from '@shared/lib/quantity';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { EmptyValue } from '@shared/ui/empty-value';
import { ExpiryLabel } from '@shared/ui/expiry-label';
import { Icon } from '@shared/ui/icon';
import { InfoNote } from '@shared/ui/info-note';
import { ListPagination, usePagedSearch } from '@shared/ui/list-pagination';
import { PanelHeader } from '@shared/ui/panel-header';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import { explainReason } from '../model/explain';
import {
	HiddenOffers,
	ManualNote,
	OfferMenu,
	type HotOfferActions,
} from './hot-offers';
import { useHotOffers } from './use-hot-offers';
import { toHotDishParams, type HotSearch } from '../model/search';

interface HotPageProps {
	pointId: string;
	search: HotSearch;
	onSearchChange: (patch: Partial<HotSearch>) => void;
}

const longestHorizon = hourWindows.at(-1)!;

interface ReasonsProps {
	dish: HotDish;
	pointId: string;
	linked: boolean;
	now: Date;
}

/** Продукты под риском блюда: сколько, когда истекает и почему это блюдо стоит продать. */
function Reasons({ dish, pointId, linked, now }: ReasonsProps) {
	return (
		<Stack component='ul' sx={{ listStyle: 'none', m: 0, p: 0, gap: 1 }}>
			{dish.reasons.map((reason) => (
				<Box component='li' key={reason.product.id}>
					<Stack
						direction='row'
						sx={{ gap: 1, alignItems: 'center', flexWrap: 'wrap' }}
					>
						<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
							<ProductHistoryLink
								product={reason.product}
								pointId={pointId}
								linked={linked}
							/>
						</Typography>
						<Typography>
							{formatAmount(reason.at_risk_quantity, reason.product.unit)} под
							риском
						</Typography>
						<ExpiryLabel
							expiresAt={reason.expires_at}
							now={now}
							lead='remaining'
							inline
						/>
					</Stack>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{explainReason(dish, reason)}
					</Typography>
				</Box>
			))}
		</Stack>
	);
}

interface HotViewProps {
	dishes: HotDish[];
	pointId: string;
	linked: boolean;
	now: Date;
	/** Действия управляющего; без права управлять меню их нет. */
	actions?: HotOfferActions;
}

/** Причина в строке: комментарий закрепившего и продукты под риском. */
function Why({ dish, ...rest }: ReasonsProps) {
	return (
		<Stack sx={{ gap: 1 }}>
			{dish.manual && <ManualNote manual={dish.manual} now={rest.now} />}
			{dish.reasons.length > 0 && <Reasons dish={dish} {...rest} />}
		</Stack>
	);
}

function DishName({ dish }: { dish: HotDish }) {
	return (
		<Stack sx={{ gap: 0.5, alignItems: 'flex-start' }}>
			<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
				{dish.dish.name}
			</Typography>
			{dish.manual && <PinnedChip />}
		</Stack>
	);
}

function HotTable({ dishes, actions, ...rest }: HotViewProps) {
	return (
		<Table aria-label='Горячие предложения' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell sx={{ width: 220 }}>Блюдо</TableCell>
					<TableCell>Почему стоит продать</TableCell>
					<TableCell sx={{ width: 130 }}>Стоит продать</TableCell>
					<TableCell sx={{ width: 160 }}>Можно приготовить</TableCell>
					{actions && <TableCell sx={{ width: 64 }} />}
				</TableRow>
			</TableHead>
			<TableBody>
				{dishes.map((dish) => (
					<TableRow key={dish.dish.id} sx={{ verticalAlign: 'top' }}>
						<TableCell>
							<DishName dish={dish} />
						</TableCell>
						<TableCell>
							<Why dish={dish} {...rest} />
						</TableCell>
						<TableCell sx={{ fontWeight: 600, color: 'text.strong' }}>
							{suggestedPortionsText(dish) ?? <EmptyValue />}
						</TableCell>
						<TableCell>{portionsText(dish.available_portions)}</TableCell>
						{actions && (
							<TableCell align='right' sx={{ py: 1 }}>
								<OfferMenu dish={dish} actions={actions} />
							</TableCell>
						)}
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function HotCards({ dishes, actions, ...rest }: HotViewProps) {
	return (
		<CardList label='Горячие предложения'>
			{dishes.map((dish) => {
				const suggested = suggestedPortionsText(dish);

				return (
					<CardListItem key={dish.dish.id} sx={{ display: 'block' }}>
						<Stack
							direction='row'
							sx={{ gap: 1.5, alignItems: 'flex-start', mb: 1 }}
						>
							<Box sx={{ flex: 1, minWidth: 0 }}>
								<DishName dish={dish} />
							</Box>
							<Box sx={{ textAlign: 'right' }}>
								{suggested && (
									<Typography sx={{ fontWeight: 600, color: 'text.strong' }}>
										продать {suggested}
									</Typography>
								)}
								<Typography variant='caption' sx={{ color: 'text.secondary' }}>
									можно {dish.available_portions}
								</Typography>
							</Box>
							{actions && <OfferMenu dish={dish} actions={actions} />}
						</Stack>
						<Why dish={dish} {...rest} />
					</CardListItem>
				);
			})}
		</CardList>
	);
}

/** Вкладка «Горячие предложения»: блюда, которые стоит продать первыми, с причиной по каждому продукту. */
export function HotPage({ pointId, search, onSearchChange }: HotPageProps) {
	const wide = useWideLayout();
	const now = useNow();
	const { profile, point } = usePoint(pointId);
	const linked = can(point, 'inventory.view');
	const manage = mayWrite(profile, point, 'menu.manage');
	const offers = useHotOffers(pointId);
	const actions = manage ? offers.actions : undefined;
	const list = useQuery(
		hotDishesQueryOptions(pointId, toHotDishParams(search))
	);

	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	const content = (data: NonNullable<typeof list.data>) => {
		if (data.items.length === 0) {
			return (
				<EmptyState
					icon='done'
					title='Продвигать нечего'
					description={`За ${hourWindowLabels[search.horizon]} у блюд меню не истекает ни одна партия`}
					action={
						search.horizon !== longestHorizon && (
							<Button
								variant='outlined'
								onClick={() => filter({ horizon: longestHorizon })}
							>
								Показать на {hourWindowLabels[longestHorizon]}
							</Button>
						)
					}
				/>
			);
		}

		const view = { dishes: data.items, pointId, linked, now, actions };

		return (
			<>
				{wide ? <HotTable {...view} /> : <HotCards {...view} />}
				<ListPagination total={data.total} {...pagination} />
			</>
		);
	};

	return (
		<>
			<InfoNote sx={{ mb: 1.5 }}>
				Сначала блюда, которые закрепил управляющий, затем блюда с продуктами,
				срок которых скоро истечёт. Продайте их первыми, чтобы не списывать
				продукты. Скрытое блюдо вернётся в расчёт завтра. Список обновляется
				сам.
			</InfoNote>
			<Paper sx={{ overflow: 'hidden' }}>
				<PanelHeader>
					<Stack
						direction='row'
						sx={{
							gap: 1.5,
							alignItems: 'center',
							flexWrap: 'wrap',
							width: '100%',
						}}
					>
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							Истекает в пределах
						</Typography>
						<SegmentedControl
							label='Истекает в пределах'
							options={hourWindows.map((value) => ({
								value: String(value),
								label: hourWindowLabels[value],
							}))}
							value={String(search.horizon)}
							onChange={(value) =>
								filter({ horizon: Number(value) as HourWindow })
							}
						/>
						<Box sx={{ flexGrow: 1 }} />
						{actions && (
							<Button
								variant='outlined'
								startIcon={<Icon name='add' />}
								onClick={actions.add}
							>
								Добавить блюдо
							</Button>
						)}
					</Stack>
				</PanelHeader>
				<QueryState
					query={list}
					loading={
						<SkeletonList
							label='Загружаем предложения'
							count={4}
							height={56}
							sx={{ p: 2 }}
						/>
					}
					errorTitle='Не удалось загрузить предложения'
				>
					{content}
				</QueryState>
				{actions && <HiddenOffers pointId={pointId} actions={actions} />}
			</Paper>
			{offers.elements}
		</>
	);
}
