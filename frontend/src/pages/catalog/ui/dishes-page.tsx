import {
	Box,
	Button,
	Chip,
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

import { companyDishesQueryOptions, type Dish } from '@entities/dish';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { EmptyState, NoResultsState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { usePagedSearch } from '@shared/ui/list-pagination';
import { useNotice } from '@shared/ui/notice';
import { SearchField } from '@shared/ui/search-field';
import { SegmentedControl } from '@shared/ui/segmented-control';

import { dishCountQueryOptions } from '../api/catalog-counts';
import { useDishArchived } from '../api/catalog-writes';
import { useCatalogScope } from '../model/catalog-scope';
import { archivedProducts, recipeLine } from '../model/recipe';
import { restoreNotice } from '../model/restore-notice';
import {
	dishFilters,
	toDishParams,
	type DishesSearch,
	type DishFilter,
} from '../model/search';
import {
	CatalogListBody,
	CatalogListPanel,
	ItemName,
	RowActions,
	type ItemActions,
} from './catalog-list';
import { DishPanel } from './dish-panel';
import { NoticeDialog } from './notice-dialog';

interface DishesPageProps {
	search: DishesSearch;
	onSearchChange: (
		patch: Partial<DishesSearch>,
		options?: { replace?: boolean }
	) => void;
}

interface RowsProps extends ItemActions<Dish> {
	items: Dish[];
}

function Recipe({ dish }: { dish: Dish }) {
	if (dish.ingredients.length === 0) {
		return <Chip color='warning' label='нет рецептуры — блюдо в стоп-листе' />;
	}

	return (
		<Typography variant='body2' sx={{ color: 'text.secondary' }}>
			{recipeLine(dish)}
		</Typography>
	);
}

function DishesTable(rows: RowsProps) {
	return (
		<Table aria-label='Блюда каталога' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell sx={{ width: '30%' }}>Блюдо</TableCell>
					<TableCell>Рецептура на порцию</TableCell>
					<TableCell sx={{ width: 180 }} />
				</TableRow>
			</TableHead>
			<TableBody>
				{rows.items.map((dish) => (
					<TableRow key={dish.id}>
						<TableCell>
							<ItemName name={dish.name} onOpen={() => rows.onEdit(dish)} />
						</TableCell>
						<TableCell>
							<Recipe dish={dish} />
						</TableCell>
						<TableCell align='right'>
							<RowActions item={dish} {...rows} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function DishCards(rows: RowsProps) {
	return (
		<CardList label='Блюда каталога'>
			{rows.items.map((dish) => (
				<CardListItem key={dish.id}>
					<Stack
						sx={{ flex: 1, minWidth: 0, gap: 0.5, alignItems: 'flex-start' }}
					>
						<ItemName name={dish.name} onOpen={() => rows.onEdit(dish)} />
						<Recipe dish={dish} />
					</Stack>
					<RowActions item={dish} {...rows} />
				</CardListItem>
			))}
		</CardList>
	);
}

const filterLabels: Record<DishFilter, string> = {
	active: 'Действующие',
	missing: 'Без рецептуры',
	archived: 'Архив',
};

function emptyTitle(filter: DishFilter): string {
	if (filter === 'missing') {
		return 'У всех блюд есть рецептура';
	}

	return filter === 'archived' ? 'В архиве пусто' : 'Блюд пока нет';
}

/** Вкладка «Блюда»: рецептуры строкой, поиск и фильтр на сервере, панель блюда по адресу. */
export function DishesPage({ search, onSearchChange }: DishesPageProps) {
	const scope = useCatalogScope();
	const { companyId } = scope;
	const list = useQuery(
		companyDishesQueryOptions(companyId, toDishParams(search))
	);
	const counts = {
		active: useQuery(dishCountQueryOptions(companyId, 'active')).data?.total,
		missing: useQuery(dishCountQueryOptions(companyId, 'missing')).data?.total,
		archived: useQuery(dishCountQueryOptions(companyId, 'archived')).data
			?.total,
	};
	const archive = useDishArchived();
	const restore = useDishArchived();
	const [archiving, setArchiving] = useState<Dish | null>(null);
	const [refused, setRefused] = useState<Dish | null>(null);
	const notice = useNotice();
	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	// Блюдо с архивным продуктом сервер не восстановит: отказ показывается сразу.
	const onRestore = (dish: Dish) => {
		if (archivedProducts(dish).length > 0) {
			setRefused(dish);
			return;
		}

		restore.mutate(
			{ id: dish.id, archived: false },
			restoreNotice(dish.name, notice.show)
		);
	};
	const restoringId = restore.isPending ? restore.variables.id : undefined;

	const actions: ItemActions<Dish> = {
		readOnly: scope.readOnly,
		restoringId,
		onEdit: (dish) => onSearchChange({ dish: dish.id }),
		onArchive: setArchiving,
		onRestore,
	};

	const empty = search.q.trim() ? (
		<NoResultsState
			description={`Нет блюд с «${search.q.trim()}» в названии`}
			onReset={() => filter({ q: '' })}
		/>
	) : (
		<EmptyState
			icon='recipe'
			title={emptyTitle(search.filter)}
			action={
				search.filter === 'active' &&
				!scope.readOnly && (
					<Button
						variant='contained'
						startIcon={<Icon name='add' />}
						onClick={scope.newDish}
					>
						Новое блюдо
					</Button>
				)
			}
		/>
	);

	return (
		<>
			<CatalogListPanel
				filters={
					<>
						<SearchField
							placeholder='Название блюда'
							label='Поиск по блюду'
							value={search.q}
							onChange={(q) => filter({ q })}
						/>
						<Box sx={{ flexGrow: 1 }} />
						<SegmentedControl
							label='Какие блюда показать'
							options={dishFilters.map((value) => ({
								value,
								label: filterLabels[value],
								count: counts[value],
							}))}
							value={search.filter}
							onChange={(next) => filter({ filter: next })}
						/>
					</>
				}
			>
				<CatalogListBody
					query={list}
					loadingLabel='Загружаем блюда'
					errorTitle='Не удалось загрузить блюда'
					empty={empty}
					table={(items) => <DishesTable items={items} {...actions} />}
					cards={(items) => <DishCards items={items} {...actions} />}
					pagination={pagination}
				/>
			</CatalogListPanel>
			<DishPanel
				dishId={search.dish}
				onClose={() => onSearchChange({ dish: undefined })}
				onArchive={setArchiving}
				onRestore={onRestore}
				restoringId={restoringId}
			/>
			{archiving && (
				<ConfirmDialog
					open
					title={`Отправить «${archiving.name}» в архив?`}
					description='Блюдо пропадёт из меню и стоп-листа всех точек, заказать его будет нельзя. Выключение блюда на точках сохранится до восстановления.'
					confirmLabel='В архив'
					pending={archive.isPending}
					error={archive.error?.message}
					onConfirm={() =>
						archive.mutate(
							{ id: archiving.id, archived: true },
							{
								onSuccess: () => {
									setArchiving(null);
									onSearchChange({ dish: undefined }, { replace: true });
								},
							}
						)
					}
					onClose={() => {
						archive.reset();
						setArchiving(null);
					}}
				/>
			)}
			{refused && (
				<NoticeDialog
					title='Нельзя восстановить блюдо'
					onClose={() => setRefused(null)}
				>
					<Typography>
						В рецептуре «{refused.name}» есть продукты из архива:{' '}
						<b>{archivedProducts(refused).join(', ')}</b>.
					</Typography>
					<Typography sx={{ mt: 1.5, color: 'text.secondary' }}>
						Восстановите продукты или уберите их из рецептуры.
					</Typography>
				</NoticeDialog>
			)}
			{notice.element}
		</>
	);
}
