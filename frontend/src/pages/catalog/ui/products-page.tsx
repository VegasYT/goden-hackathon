import {
	Box,
	Button,
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
	ProductCategorySelect,
	productListQueryOptions,
	shelfLifeDays,
	type Product,
} from '@entities/product';
import { countOf } from '@shared/lib/plural';
import { quantityInput } from '@shared/lib/quantity';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyState, NoResultsState } from '@shared/ui/empty-state';
import { EmptyValue } from '@shared/ui/empty-value';
import { Icon } from '@shared/ui/icon';
import { usePagedSearch } from '@shared/ui/list-pagination';
import { useNotice } from '@shared/ui/notice';
import { SearchField } from '@shared/ui/search-field';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { SortableHeaderCell } from '@shared/ui/sortable-header-cell';

import { productCountQueryOptions } from '../api/catalog-counts';
import { useProductArchived } from '../api/catalog-writes';
import { useCatalogScope } from '../model/catalog-scope';
import { restoreNotice } from '../model/restore-notice';
import {
	isProductsFiltered,
	toProductParams,
	type ProductsSearch,
} from '../model/search';
import { ArchiveProductDialog } from './archive-product-dialog';
import {
	CatalogListBody,
	CatalogListPanel,
	ItemName,
	RowActions,
	type ItemActions,
} from './catalog-list';
import { ProductDialog } from './product-dialog';

interface ProductsPageProps {
	search: ProductsSearch;
	onSearchChange: (patch: Partial<ProductsSearch>) => void;
}

function dishesText(dishes: number): string {
	return countOf(dishes, 'блюдо', 'блюда', 'блюд');
}

function shelfLifeText(product: Product): string {
	return product.shelf_life_days === undefined
		? 'без срока'
		: shelfLifeDays(product.shelf_life_days);
}

function notFoundText(search: ProductsSearch): string {
	const query = search.q.trim();
	const where = search.archived ? 'в архиве ' : '';

	if (search.category && query) {
		return `В категории «${search.category}» ${where}нет продуктов с «${query}» в названии`;
	}

	if (query) {
		return `${where ? 'В архиве нет' : 'Нет'} продуктов с «${query}» в названии`;
	}

	return `В категории «${search.category}» ${where}нет продуктов`;
}

interface RowsProps extends ItemActions<Product> {
	items: Product[];
}

function Name({ product, readOnly, onEdit }: RowsProps & { product: Product }) {
	return (
		<ItemName
			name={product.name}
			onOpen={readOnly || product.archived ? undefined : () => onEdit(product)}
		/>
	);
}

function ProductsTable({
	sort,
	onSortChange,
	...rows
}: RowsProps & {
	sort: ProductsSearch['sort'];
	onSortChange: (sort: ProductsSearch['sort']) => void;
}) {
	const sortable = (field: ProductsSearch['sort']) => ({
		direction: sort === field ? ('asc' as const) : undefined,
		onSort: () => onSortChange(field),
	});

	return (
		<Table aria-label='Продукты каталога' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<SortableHeaderCell {...sortable('name')}>Продукт</SortableHeaderCell>
					<SortableHeaderCell sx={{ width: '22%' }} {...sortable('category')}>
						Категория
					</SortableHeaderCell>
					<TableCell sx={{ width: 100 }}>Единица</TableCell>
					<TableCell sx={{ width: 150 }}>Срок хранения</TableCell>
					<TableCell sx={{ width: 130 }}>В рецептурах</TableCell>
					<TableCell sx={{ width: 180 }} />
				</TableRow>
			</TableHead>
			<TableBody>
				{rows.items.map((product) => (
					<TableRow key={product.id}>
						<TableCell>
							<Name product={product} {...rows} />
						</TableCell>
						<TableCell>{product.category || <EmptyValue />}</TableCell>
						<TableCell>{quantityInput(product.unit).unit}</TableCell>
						<TableCell>{shelfLifeText(product)}</TableCell>
						<TableCell>
							{product.dishes ? dishesText(product.dishes) : <EmptyValue />}
						</TableCell>
						<TableCell align='right'>
							<RowActions item={product} {...rows} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function ProductCards(rows: RowsProps) {
	return (
		<CardList label='Продукты каталога'>
			{rows.items.map((product) => (
				<CardListItem key={product.id}>
					<Stack
						sx={{ flex: 1, minWidth: 0, gap: 0.5, alignItems: 'flex-start' }}
					>
						<Name product={product} {...rows} />
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							{[
								product.category,
								quantityInput(product.unit).unit,
								shelfLifeText(product),
								product.dishes ? `в ${dishesText(product.dishes)}` : undefined,
							]
								.filter(Boolean)
								.join(' · ')}
						</Typography>
					</Stack>
					<RowActions item={product} {...rows} />
				</CardListItem>
			))}
		</CardList>
	);
}

/** Вкладка «Продукты»: поиск, категория, архив и сортировка считаются на сервере. */
export function ProductsPage({ search, onSearchChange }: ProductsPageProps) {
	const scope = useCatalogScope();
	const { companyId } = scope;
	const list = useQuery(
		productListQueryOptions(companyId, toProductParams(search))
	);
	const active = useQuery(productCountQueryOptions(companyId, false));
	const archived = useQuery(productCountQueryOptions(companyId, true));
	const restore = useProductArchived();
	const [editing, setEditing] = useState<Product | null>(null);
	const [archiving, setArchiving] = useState<Product | null>(null);
	const notice = useNotice();
	const { filter, pagination } = usePagedSearch(
		search,
		onSearchChange,
		list.data?.total
	);

	const actions: ItemActions<Product> = {
		readOnly: scope.readOnly,
		restoringId: restore.isPending ? restore.variables.id : undefined,
		onEdit: setEditing,
		onArchive: setArchiving,
		onRestore: (product) =>
			restore.mutate(
				{ id: product.id, archived: false },
				restoreNotice(product.name, notice.show)
			),
	};

	const empty = isProductsFiltered(search) ? (
		<NoResultsState
			description={notFoundText(search)}
			resetLabel='Сбросить фильтры'
			onReset={() => filter({ q: '', category: '' })}
		/>
	) : search.archived ? (
		<EmptyState icon='archive' title='В архиве пусто' />
	) : (
		<EmptyState
			icon='catalog'
			title='Каталог пока пустой'
			description='Добавьте продукты, которые закупает ресторан, затем блюда с рецептурами. Каталог общий для всех точек компании.'
			action={
				!scope.readOnly && (
					<Button
						variant='contained'
						startIcon={<Icon name='add' />}
						onClick={scope.newProduct}
					>
						Новый продукт
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
							placeholder='Название продукта'
							label='Поиск по продукту'
							value={search.q}
							onChange={(q) => filter({ q })}
						/>
						<ProductCategorySelect
							companyId={companyId}
							value={search.category}
							onChange={(category) => filter({ category })}
						/>
						<Box sx={{ flexGrow: 1 }} />
						<SegmentedControl
							label='Действующие или архив'
							options={[
								{
									value: 'active',
									label: 'Действующие',
									count: active.data?.total,
								},
								{
									value: 'archived',
									label: 'Архив',
									count: archived.data?.total,
								},
							]}
							value={search.archived ? 'archived' : 'active'}
							onChange={(next) => filter({ archived: next === 'archived' })}
						/>
					</>
				}
			>
				<CatalogListBody
					query={list}
					loadingLabel='Загружаем продукты'
					errorTitle='Не удалось загрузить продукты'
					empty={empty}
					table={(items) => (
						<ProductsTable
							items={items}
							{...actions}
							sort={search.sort}
							onSortChange={(sort) => filter({ sort })}
						/>
					)}
					cards={(items) => <ProductCards items={items} {...actions} />}
					pagination={pagination}
				/>
			</CatalogListPanel>
			{editing && (
				<ProductDialog
					product={editing}
					onClose={() => setEditing(null)}
					onSaved={() => setEditing(null)}
				/>
			)}
			{archiving && (
				<ArchiveProductDialog
					product={archiving}
					onClose={() => setArchiving(null)}
				/>
			)}
			{notice.element}
		</>
	);
}
