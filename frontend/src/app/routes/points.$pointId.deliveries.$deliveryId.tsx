import { createFileRoute, Outlet } from '@tanstack/react-router';

import { DeliveryLoading, DeliveryNotFound } from '@pages/delivery';

import { ensureDelivery } from '../delivery-access';

export const Route = createFileRoute('/points/$pointId/deliveries/$deliveryId')(
	{
		loader: ({ context, params }) =>
			ensureDelivery(context.queryClient, params.pointId, params.deliveryId),
		component: Outlet,
		pendingComponent: DeliveryLoading,
		notFoundComponent: function DeliveryNotFoundRoute() {
			return <DeliveryNotFound pointId={Route.useParams().pointId} />;
		},
	}
);
