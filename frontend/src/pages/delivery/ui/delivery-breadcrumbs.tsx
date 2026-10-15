import type { Delivery } from '@entities/delivery';
import { BreadcrumbLink, Breadcrumbs } from '@shared/ui/breadcrumbs';

/** Путь «Поставки › № 118», а на вложенной странице — «Поставки › № 118 › <страница>». */
export function DeliveryBreadcrumbs({
	pointId,
	delivery,
	page,
}: {
	pointId: string;
	delivery: Delivery;
	page?: string;
}) {
	const number = `№ ${delivery.number}`;
	const deliveries = (
		<BreadcrumbLink to='/points/$pointId/deliveries' params={{ pointId }}>
			Поставки
		</BreadcrumbLink>
	);

	if (!page) {
		return <Breadcrumbs current={number}>{deliveries}</Breadcrumbs>;
	}

	return (
		<Breadcrumbs current={page}>
			{deliveries}
			<BreadcrumbLink
				to='/points/$pointId/deliveries/$deliveryId'
				params={{ pointId, deliveryId: delivery.id }}
			>
				{number}
			</BreadcrumbLink>
		</Breadcrumbs>
	);
}
