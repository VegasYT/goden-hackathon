import { Skeleton, Stack } from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import { productListQueryOptions } from '@entities/product';
import { isPointDisabled, usePoint } from '@entities/profile';
import { BreadcrumbLink, Breadcrumbs } from '@shared/ui/breadcrumbs';
import { EmptyState, LoadErrorState } from '@shared/ui/empty-state';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { ReadOnlyNote } from '@shared/ui/read-only-note';

import { RegistrationForm } from './registration-form';

/** Регистрация новой поставки по накладной. */
export function DeliveryNewPage({ pointId }: { pointId: string }) {
	const { profile, point } = usePoint(pointId);
	const companyId = point.company_id;
	// Пустой каталог узнаётся по числу продуктов: поле продукта ищет их само.
	const products = useQuery(
		productListQueryOptions(companyId, { limit: 1, offset: 0 })
	);

	const header = (
		<PageHeader
			breadcrumbs={
				<Breadcrumbs current='Новая поставка'>
					<BreadcrumbLink to='/points/$pointId/deliveries' params={{ pointId }}>
						Поставки
					</BreadcrumbLink>
				</Breadcrumbs>
			}
			title='Новая поставка'
			meta='Внесите поставку по накладной поставщика. Принять её можно сразу после регистрации.'
		/>
	);

	if (isPointDisabled(profile, point)) {
		return (
			<PageContent>
				{header}
				<ReadOnlyNote>
					Точка отключена — поставки на ней не регистрируются.
				</ReadOnlyNote>
			</PageContent>
		);
	}

	if (products.isPending) {
		return (
			<PageContent>
				{header}
				<Stack role='status' aria-label='Загружаем каталог' sx={{ gap: 1.5 }}>
					<Skeleton variant='rounded' height={88} />
					<Skeleton variant='rounded' height={240} />
				</Stack>
			</PageContent>
		);
	}

	if (products.isError) {
		return (
			<PageContent>
				{header}
				<LoadErrorState title='Не удалось загрузить каталог' query={products} />
			</PageContent>
		);
	}

	if (products.data.total === 0) {
		return (
			<PageContent>
				{header}
				<EmptyState
					icon='empty'
					title='Зарегистрировать поставку пока нельзя'
					description='В каталоге компании нет продуктов. Их добавляет владелец компании.'
				/>
			</PageContent>
		);
	}

	return (
		<RegistrationForm pointId={pointId} companyId={companyId} header={header} />
	);
}
