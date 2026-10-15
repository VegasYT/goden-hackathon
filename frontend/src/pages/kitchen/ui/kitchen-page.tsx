import { Alert, Box, MenuItem, Paper, TextField } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import {
	EmptyMenuState,
	menuQueryOptions,
	pointDishesQueryOptions,
	stoppedDishes,
	type DishAvailability,
	type DishRef,
} from '@entities/dish';
import { AccessNote, usePoint, writeAccess } from '@entities/profile';
import { countOf } from '@shared/lib/plural';
import { useToggleSet } from '@shared/lib/toggle-set';
import { NoResultsState, QueryState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { PanelHeader } from '@shared/ui/panel-header';
import { SearchField } from '@shared/ui/search-field';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import { disabledDishes, visibleMenu } from '../model/menu';
import type { KitchenSearch, KitchenSort } from '../model/search';
import { DisableDialog } from './dish-actions';
import { MenuCards, MenuTable } from './menu-views';

interface KitchenPageProps {
	pointId: string;
	search: KitchenSearch;
	onSearchChange: (patch: Partial<KitchenSearch>) => void;
}

const sortLabels: Record<KitchenSort, string> = {
	name: 'По названию',
	portions: 'Меньше порций сначала',
};

function menuSummary(menu: DishAvailability[]): string {
	return `${countOf(menu.length, 'блюдо', 'блюда', 'блюд')} в меню, ${stoppedDishes(menu).length} в стоп-листе`;
}

/** Меню точки: порции, рецептуры, выключенные блюда; поиск и сортировка хранятся в адресе. */
export function KitchenPage({
	pointId,
	search,
	onSearchChange,
}: KitchenPageProps) {
	const wide = useWideLayout();
	const { profile, point } = usePoint(pointId);
	const access = writeAccess(profile, point, 'menu.manage');
	const mayManage = access === 'allowed';
	const menu = useQuery(menuQueryOptions(pointId));
	const dishes = useQuery(pointDishesQueryOptions(pointId));
	const [expanded, toggle] = useToggleSet();
	const [disabling, setDisabling] = useState<DishRef>();
	const [enableError, setEnableError] = useState<string | null>(null);
	const searching = search.q.trim() !== '';
	const shown = menu.data ? visibleMenu(menu.data, search.q, search.sort) : [];
	const disabled = disabledDishes(dishes.data ?? [], search.q);
	const nothingShown = shown.length === 0 && disabled.length === 0;

	const content = (data: DishAvailability[]) => {
		if (searching && nothingShown) {
			return (
				<NoResultsState
					title={`Ничего не нашлось по «${search.q.trim()}»`}
					description='Поиск идёт по названию блюда и продуктам рецептуры'
					onReset={() => onSearchChange({ q: '' })}
				/>
			);
		}

		const props = {
			pointId,
			menu: shown,
			disabled,
			expanded,
			onToggle: toggle,
			mayManage,
			onDisable: setDisabling,
			onEnableError: setEnableError,
		};

		return (
			<>
				{data.length === 0 && !searching && (
					<EmptyMenuState hint='Как только они появятся, здесь будет видно, сколько порций можно приготовить.' />
				)}
				{!nothingShown &&
					(wide ? <MenuTable {...props} /> : <MenuCards {...props} />)}
			</>
		);
	};

	return (
		<PageContent>
			<PageHeader title='Кухня' meta={menu.data && menuSummary(menu.data)} />
			{enableError && (
				<Alert
					severity='error'
					onClose={() => setEnableError(null)}
					sx={{ mt: 2.5 }}
				>
					{enableError}
				</Alert>
			)}
			{access !== 'allowed' && (
				<Box sx={{ mt: 2.5 }}>
					<AccessNote
						access={access}
						disabled='Точка отключена — выключать и включать блюда нельзя'
						forbidden='Выключать и включать блюда могут управляющий и владелец'
					/>
				</Box>
			)}
			<Paper sx={{ mt: 2.5, overflow: 'hidden' }}>
				<PanelHeader>
					<SearchField
						placeholder='Блюдо или продукт'
						label='Поиск по блюду или продукту'
						value={search.q}
						onChange={(q) => onSearchChange({ q })}
					/>
					<TextField
						select
						value={search.sort}
						onChange={(event) =>
							onSearchChange({ sort: event.target.value as KitchenSort })
						}
						slotProps={{
							htmlInput: { 'aria-label': 'Сортировка меню' },
							input: {
								startAdornment: (
									<Icon name='sort' sx={{ color: 'text.secondary', mr: 1 }} />
								),
							},
						}}
						sx={{ minWidth: 240 }}
					>
						{Object.entries(sortLabels).map(([value, label]) => (
							<MenuItem key={value} value={value}>
								{label}
							</MenuItem>
						))}
					</TextField>
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
			{disabling && (
				<DisableDialog
					pointId={pointId}
					dish={disabling}
					onClose={() => setDisabling(undefined)}
				/>
			)}
		</PageContent>
	);
}
