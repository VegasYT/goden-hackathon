import { Skeleton, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import { productQueryOptions, type Product } from '@entities/product';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { InfoNote } from '@shared/ui/info-note';

import { useProductArchived } from '../api/catalog-writes';
import { useCatalogScope } from '../model/catalog-scope';
import { NoticeDialog } from './notice-dialog';

/** Отказ: продукт есть в рецептурах действующих блюд. Блюда берутся из продукта по идентификатору. */
function InUseNotice({
	product,
	onClose,
}: {
	product: Product;
	onClose: () => void;
}) {
	const { companyId } = useCatalogScope();
	const detail = useQuery(productQueryOptions(companyId, product.id));
	const dishes = detail.data?.used_in ?? [];

	return (
		<NoticeDialog title='Нельзя отправить в архив' onClose={onClose}>
			<Typography sx={{ mb: 1 }}>
				«{product.name}» есть в рецептурах блюд, которые продаются:
			</Typography>
			{detail.isPending ? (
				<Skeleton width='60%' />
			) : (
				<Typography component='ul' sx={{ m: 0, pl: 3 }}>
					{dishes.map((dish) => (
						<li key={dish.id}>{dish.name}</li>
					))}
				</Typography>
			)}
			<Typography sx={{ mt: 1.5, color: 'text.secondary' }}>
				Уберите продукт из рецептур или сначала отправьте блюда в архив.
			</Typography>
		</NoticeDialog>
	);
}

/** Архив продукта: подтверждение или отказ, если продукт есть в рецептурах. */
export function ArchiveProductDialog({
	product,
	onClose,
}: {
	product: Product;
	onClose: () => void;
}) {
	const archive = useProductArchived();

	if ((product.dishes ?? 0) > 0) {
		return <InUseNotice product={product} onClose={onClose} />;
	}

	return (
		<ConfirmDialog
			open
			title={`Отправить «${product.name}» в архив?`}
			description='Продукт пропадёт из выбора в новых поставках и рецептурах. Остатки на складе останутся, пока их не израсходуют или не спишут.'
			confirmLabel='В архив'
			pending={archive.isPending}
			error={archive.error?.message}
			onConfirm={() =>
				archive.mutate(
					{ id: product.id, archived: true },
					{ onSuccess: onClose }
				)
			}
			onClose={onClose}
		>
			<InfoNote>Вернуть продукт можно в каталоге, в «Архиве».</InfoNote>
		</ConfirmDialog>
	);
}
