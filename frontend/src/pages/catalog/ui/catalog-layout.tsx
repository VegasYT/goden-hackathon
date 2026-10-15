import { Box, Button } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { isCompanyDisabled, usePoint } from '@entities/profile';
import { TabLabel } from '@shared/ui/count-badge';
import { Icon } from '@shared/ui/icon';
import { SectionTabs, TabLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { ReadOnlyNote } from '@shared/ui/read-only-note';

import {
	dishCountQueryOptions,
	productCountQueryOptions,
} from '../api/catalog-counts';
import { CatalogScopeContext, type CatalogScope } from '../model/catalog-scope';
import { NewDishDialog } from './new-dish-dialog';
import { ProductDialog } from './product-dialog';

export type CatalogTab = 'products' | 'dishes';

/** Каталог компании открытой точки: заголовок, вкладки «Продукты» и «Блюда», диалоги создания. */
export function CatalogLayout({
	pointId,
	tab,
	children,
}: {
	pointId: string;
	tab: CatalogTab;
	children: ReactNode;
}) {
	const { profile, point } = usePoint(pointId);
	const companyId = point.company_id;
	const products = useQuery(productCountQueryOptions(companyId, false));
	const dishes = useQuery(dishCountQueryOptions(companyId, 'active'));
	const [newProduct, setNewProduct] = useState(false);
	const [newDish, setNewDish] = useState(false);
	const readOnly = isCompanyDisabled(profile, companyId);

	const scope: CatalogScope = {
		pointId,
		companyId,
		readOnly,
		newProduct: () => setNewProduct(true),
		newDish: () => setNewDish(true),
	};

	const action =
		!readOnly &&
		(tab === 'products' ? (
			<Button
				variant='contained'
				startIcon={<Icon name='add' />}
				onClick={scope.newProduct}
			>
				Новый продукт
			</Button>
		) : (
			<Button
				variant='contained'
				startIcon={<Icon name='add' />}
				onClick={scope.newDish}
			>
				Новое блюдо
			</Button>
		));

	return (
		<CatalogScopeContext value={scope}>
			<PageContent>
				<PageHeader
					title='Каталог'
					meta={`Общий для всех точек компании «${point.company_name}»`}
					actions={action}
				/>
				<SectionTabs value={tab} label='Разделы каталога'>
					<TabLink
						value='products'
						label={<TabLabel text='Продукты' count={products.data?.total} />}
						to='/points/$pointId/catalog'
						params={{ pointId }}
					/>
					<TabLink
						value='dishes'
						label={<TabLabel text='Блюда' count={dishes.data?.total} />}
						to='/points/$pointId/catalog/dishes'
						params={{ pointId }}
					/>
				</SectionTabs>
				{readOnly && (
					<Box sx={{ mb: 1.5 }}>
						<ReadOnlyNote>
							Компания отключена — каталог доступен только для чтения
						</ReadOnlyNote>
					</Box>
				)}
				{children}
			</PageContent>
			{newProduct && (
				<ProductDialog
					onClose={() => setNewProduct(false)}
					onSaved={() => setNewProduct(false)}
				/>
			)}
			{newDish && <NewDishDialog onClose={() => setNewDish(false)} />}
		</CatalogScopeContext>
	);
}
