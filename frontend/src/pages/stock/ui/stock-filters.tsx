import { Button, MenuItem, Stack, TextField } from '@mui/material';
import { useState } from 'react';

import { ProductCategorySelect } from '@entities/product';
import { stockSorts, type StockSort } from '@entities/stock';
import { FormDialog } from '@shared/ui/form-dialog';
import { Icon } from '@shared/ui/icon';
import { PanelHeader } from '@shared/ui/panel-header';
import { SearchField } from '@shared/ui/search-field';
import {
	SegmentedControl,
	type SegmentedOption,
} from '@shared/ui/segmented-control';

import type { StockSummary } from '../api/stock';
import type { StockSearch, StockStatusFilter } from '../model/search';

const sortLabels: Record<StockSort, string> = {
	name: 'По названию, А–Я',
	'-name': 'По названию, Я–А',
	nearest_expiry: 'Сначала ближайший срок',
	'-nearest_expiry': 'Сначала дальний срок',
	available: 'Сначала меньше доступно',
	'-available': 'Сначала больше доступно',
};

interface StockFiltersProps {
	search: StockSearch;
	summary: StockSummary | undefined;
	companyId: string;
	wide: boolean;
	/** Меняет фильтры; страница при этом сбрасывается на первую. */
	onChange: (patch: Partial<StockSearch>) => void;
}

function statusOptions(
	summary: StockSummary | undefined
): SegmentedOption<StockStatusFilter>[] {
	return [
		{ value: 'all', label: 'Все', count: summary?.total },
		{ value: 'expiring', label: 'Истекают', count: summary?.expiring },
		{ value: 'expired', label: 'Просрочено', count: summary?.expired },
		{
			value: 'out_of_stock',
			label: 'Нет в наличии',
			count: summary?.out_of_stock,
		},
	];
}

function SortSelect({
	value,
	onChange,
	fullWidth = false,
}: {
	value: StockSort;
	onChange: (sort: StockSort) => void;
	fullWidth?: boolean;
}) {
	return (
		<TextField
			select
			label={fullWidth ? 'Сортировка' : undefined}
			value={value}
			onChange={(event) => onChange(event.target.value as StockSort)}
			fullWidth={fullWidth}
			sx={fullWidth ? undefined : { width: 210 }}
			slotProps={{ htmlInput: { 'aria-label': 'Сортировка' } }}
		>
			{stockSorts.map((sort) => (
				<MenuItem key={sort} value={sort}>
					{sortLabels[sort]}
				</MenuItem>
			))}
		</TextField>
	);
}

/** Категория и сортировка на планшете: применяются по «Показать». */
function FiltersDialog({
	search,
	companyId,
	onApply,
	onClose,
}: {
	search: StockSearch;
	companyId: string;
	onApply: (patch: Pick<StockSearch, 'category' | 'sort'>) => void;
	onClose: () => void;
}) {
	const [category, setCategory] = useState(search.category);
	const [sort, setSort] = useState(search.sort);

	return (
		<FormDialog
			title='Фильтры'
			submitLabel='Показать'
			onSubmit={() => onApply({ category, sort })}
			onClose={onClose}
		>
			<ProductCategorySelect
				value={category}
				companyId={companyId}
				onChange={setCategory}
				fullWidth
			/>
			<SortSelect value={sort} onChange={setSort} fullWidth />
		</FormDialog>
	);
}

/** Поиск, категория, статус и сортировка остатков; на планшете категория и сортировка — в диалоге. */
export function StockFilters({
	search,
	summary,
	companyId,
	wide,
	onChange,
}: StockFiltersProps) {
	const [dialogOpen, setDialogOpen] = useState(false);
	const searchField = (
		<SearchField
			placeholder='Название продукта'
			label='Поиск по продукту'
			value={search.q}
			onChange={(q) => onChange({ q })}
		/>
	);
	const status = (
		<SegmentedControl
			label='Фильтр по состоянию'
			options={statusOptions(summary)}
			value={search.status}
			onChange={(next) => onChange({ status: next })}
		/>
	);

	if (wide) {
		return (
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
					{searchField}
					<ProductCategorySelect
						value={search.category}
						companyId={companyId}
						onChange={(category) => onChange({ category })}
					/>
					{status}
					<Stack sx={{ flexGrow: 1 }} />
					<SortSelect
						value={search.sort}
						onChange={(sort) => onChange({ sort })}
					/>
				</Stack>
			</PanelHeader>
		);
	}

	const changed =
		Number(search.category !== '') + Number(search.sort !== 'name');

	return (
		<PanelHeader>
			<Stack sx={{ gap: 1.5, width: '100%' }}>
				<Stack direction='row' sx={{ gap: 1, alignItems: 'center' }}>
					<Stack sx={{ flexGrow: 1 }}>{searchField}</Stack>
					<Button
						variant='outlined'
						size='large'
						startIcon={<Icon name='filters' />}
						onClick={() => setDialogOpen(true)}
					>
						{changed > 0 ? `Фильтры · ${changed}` : 'Фильтры'}
					</Button>
				</Stack>
				{status}
			</Stack>
			{dialogOpen && (
				<FiltersDialog
					search={search}
					companyId={companyId}
					onClose={() => setDialogOpen(false)}
					onApply={(patch) => {
						setDialogOpen(false);
						onChange(patch);
					}}
				/>
			)}
		</PanelHeader>
	);
}
