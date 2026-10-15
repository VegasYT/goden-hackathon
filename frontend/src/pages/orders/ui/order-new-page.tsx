import {
	Box,
	Button,
	Chip,
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

import {
	EmptyMenuState,
	hotCaption,
	menuMatcher,
	menuQueryOptions,
	PinnedChip,
	portionsText,
	stopCaption,
	StopListChip,
	type DishAvailability,
	type HotDish,
} from '@entities/dish';
import { can, usePoint, writeAccess } from '@entities/profile';
import { useNow } from '@shared/lib/expiry';
import { ActionBar } from '@shared/ui/action-bar';
import { BreadcrumbLink, Breadcrumbs } from '@shared/ui/breadcrumbs';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { NoResultsState, QueryState } from '@shared/ui/empty-state';
import { InfoNote } from '@shared/ui/info-note';
import { ButtonLink, TextLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { PanelHeader } from '@shared/ui/panel-header';
import { QuantityField } from '@shared/ui/quantity-field';
import { ReadOnlyNote } from '@shared/ui/read-only-note';
import { RetryAlert } from '@shared/ui/retry-alert';
import { SearchField } from '@shared/ui/search-field';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import { defaultHotDishesQueryOptions } from '../api/hot-dishes';
import { usePlaceOrder } from '../api/order-writes';
import {
	orderBody,
	orderLines,
	limitOf,
	overLimit,
	withoutDishes,
	type Portions,
} from '../model/order-form';
import { OrderSummary } from './order-summary';
import { ResultAlert, type OrderResult } from './result-alert';

interface RowProps {
	dish: DishAvailability;
	value: number;
	/** Горячее предложение с этим блюдом. */
	hot: HotDish | undefined;
	now: Date;
	over: boolean;
	disabled: boolean;
	onChange: (value: number) => void;
}

function DishName({ dish, hot, now }: Pick<RowProps, 'dish' | 'hot' | 'now'>) {
	return (
		<>
			<Stack
				direction='row'
				sx={{ gap: 1, alignItems: 'center', flexWrap: 'wrap' }}
			>
				<Typography
					sx={{
						fontWeight: 500,
						color: dish.available ? 'text.strong' : 'text.secondary',
					}}
				>
					{dish.dish.name}
				</Typography>
				{dish.available &&
					hot &&
					(hot.manual ? (
						<PinnedChip />
					) : (
						<Chip color='warning' label='предложить' />
					))}
			</Stack>
			<Typography
				variant='caption'
				component='div'
				sx={{ color: 'text.secondary' }}
			>
				{dish.available ? hot && hotCaption(hot, now) : stopCaption(dish)}
			</Typography>
		</>
	);
}

function PortionsField({ dish, value, over, disabled, onChange }: RowProps) {
	const limit = limitOf(dish);
	const helper = over
		? limit > 0
			? `можно только ${limit}`
			: 'сейчас не приготовить'
		: value > 0 && value >= limit
			? 'больше не приготовить'
			: undefined;

	return (
		<QuantityField
			label='В заказе'
			hideLabel
			accessibleLabel={`${dish.dish.name}: порций`}
			value={value}
			onValueChange={(next) => onChange(Math.max(0, Math.round(next ?? 0)))}
			max={Math.max(limit, value)}
			format={{ maximumFractionDigits: 0 }}
			disabled={disabled || (!dish.available && value === 0)}
			error={over}
			helperText={helper}
		/>
	);
}

function Available({ dish }: { dish: DishAvailability }) {
	return dish.available ? <>{portionsText(dish.portions)}</> : <StopListChip />;
}

interface MenuViewProps {
	rows: RowProps[];
}

function MenuTable({ rows }: MenuViewProps) {
	return (
		<Table aria-label='Меню для заказа' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Блюдо</TableCell>
					<TableCell sx={{ width: 180 }}>Можно приготовить</TableCell>
					<TableCell sx={{ width: 190 }}>В заказе</TableCell>
				</TableRow>
			</TableHead>
			<TableBody>
				{rows.map((row) => (
					<TableRow key={row.dish.dish.id} sx={{ verticalAlign: 'top' }}>
						<TableCell>
							<DishName {...row} />
						</TableCell>
						<TableCell>
							<Available dish={row.dish} />
						</TableCell>
						<TableCell>
							<PortionsField {...row} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function MenuCards({ rows }: MenuViewProps) {
	return (
		<CardList label='Меню для заказа'>
			{rows.map((row) => (
				<CardListItem key={row.dish.dish.id}>
					<Box sx={{ flex: 1, minWidth: 0 }}>
						<DishName {...row} />
						{row.dish.available && (
							<Typography variant='body2' sx={{ color: 'text.secondary' }}>
								можно {row.dish.portions}
							</Typography>
						)}
					</Box>
					{row.dish.available || row.value > 0 ? (
						<PortionsField {...row} />
					) : (
						<StopListChip />
					)}
				</CardListItem>
			))}
		</CardList>
	);
}

/** Ручное оформление заказа по меню точки: порций не больше, чем можно приготовить сейчас. */
export function OrderNewPage({ pointId }: { pointId: string }) {
	const wide = useWideLayout();
	const now = useNow();
	const { profile, point } = usePoint(pointId);
	const disabled = writeAccess(profile, point, 'orders.accept') === 'disabled';
	const menu = useQuery(menuQueryOptions(pointId));
	const hot = useQuery({
		...defaultHotDishesQueryOptions(pointId),
		enabled: can(point, 'recommendations.view'),
	});
	const [portions, setPortions] = useState<Portions>({});
	const [query, setQuery] = useState('');
	const [result, setResult] = useState<OrderResult>();
	const mutation = usePlaceOrder(pointId, {
		onPlaced: (order) => {
			if (order.status === 'accepted') {
				setPortions({});
			}

			// только что оформленный заказ не бывает отменённым
			if (order.status !== 'cancelled') {
				setResult({ kind: order.status, order });
			}
		},
		onDropped: (dropped) => {
			setPortions((current) =>
				withoutDishes(
					current,
					dropped.map((dish) => dish.id)
				)
			);
			setResult({ kind: 'dropped', dishes: dropped });
		},
	});

	const dishes = menu.data ?? [];
	const lines = orderLines(dishes, portions);
	const over = overLimit(dishes, portions);
	const body = orderBody(dishes, portions);
	const hotDishes = new Map(
		(hot.data?.items ?? []).map((item) => [item.dish.id, item])
	);
	const shown = dishes.filter(menuMatcher(query, { byIngredients: false }));

	const change = (dishId: string, value: number) => {
		setPortions((current) => ({ ...current, [dishId]: value }));

		if (result?.kind === 'accepted') {
			setResult(undefined);
		}
	};

	const submit = () => {
		if (!body) {
			return;
		}

		setResult(undefined);
		mutation.mutate({ body, dishes: lines.map((line) => line.dish) });
	};

	const rows: RowProps[] = shown.map((dish) => ({
		dish,
		value: portions[dish.dish.id] ?? 0,
		hot: hotDishes.get(dish.dish.id),
		now,
		over: over.has(dish.dish.id),
		disabled,
		onChange: (value) => change(dish.dish.id, value),
	}));

	const content = () => {
		if (dishes.length === 0) {
			return <EmptyMenuState />;
		}

		if (shown.length === 0) {
			return (
				<NoResultsState
					title={`Нет блюд с «${query.trim()}» в названии`}
					onReset={() => setQuery('')}
				/>
			);
		}

		return wide ? <MenuTable rows={rows} /> : <MenuCards rows={rows} />;
	};

	return (
		<PageContent
			actionBar={
				<ActionBar
					label='Итог заказа'
					action={
						<Stack direction='row' sx={{ gap: 1 }}>
							<ButtonLink
								to='/points/$pointId/orders'
								params={{ pointId }}
								variant='outlined'
								size='large'
							>
								Отмена
							</ButtonLink>
							<Button
								variant='contained'
								size='large'
								loading={mutation.isPending}
								disabled={!body || disabled}
								onClick={submit}
							>
								Оформить заказ
							</Button>
						</Stack>
					}
				>
					<OrderSummary lines={lines} over={over.size > 0} />
				</ActionBar>
			}
		>
			<PageHeader
				breadcrumbs={
					<Breadcrumbs current='Новый заказ'>
						<BreadcrumbLink to='/points/$pointId/orders' params={{ pointId }}>
							Заказы
						</BreadcrumbLink>
					</Breadcrumbs>
				}
				title='Новый заказ'
				meta={`${point.name} · порций не больше, чем можно приготовить сейчас`}
			/>
			<Stack sx={{ gap: 2 }}>
				{disabled && (
					<ReadOnlyNote>
						Точка отключена — заказы на ней не оформляются.
					</ReadOnlyNote>
				)}
				{result && <ResultAlert result={result} pointId={pointId} />}
				{mutation.isError && (
					<RetryAlert
						title='Заказ не оформлен'
						error={mutation.error}
						note='Если он всё же дошёл, повтор не создаст второй заказ.'
						onRetry={submit}
					/>
				)}
				{hotDishes.size > 0 && (
					<InfoNote>
						Блюда с отметкой «предложить» стоит предложить гостю: в них продукты
						с истекающим сроком; «закреплено» — блюда, которые выбрал
						управляющий. Почему — во вкладке{' '}
						<TextLink to='/points/$pointId/orders/hot' params={{ pointId }}>
							Горячие предложения
						</TextLink>
						.
					</InfoNote>
				)}
				<Paper sx={{ overflow: 'hidden' }}>
					<PanelHeader>
						<Typography variant='h3' component='h2'>
							Меню
						</Typography>
						<SearchField
							label='Поиск блюда'
							placeholder='Блюдо'
							value={query}
							onChange={setQuery}
						/>
					</PanelHeader>
					<QueryState
						query={menu}
						loading={
							<SkeletonList label='Загружаем меню' height={44} sx={{ p: 2 }} />
						}
						errorTitle='Не удалось загрузить меню'
					>
						{content}
					</QueryState>
				</Paper>
			</Stack>
		</PageContent>
	);
}
