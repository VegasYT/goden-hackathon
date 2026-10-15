import { createFileRoute } from '@tanstack/react-router';

import { DeliveryCorrectionPage } from '@pages/delivery';

import { requireCorrection } from '../delivery-access';

export const Route = createFileRoute(
	'/points/$pointId/deliveries/$deliveryId/correction'
)({
	loader: ({ context, params }) =>
		requireCorrection(context.queryClient, params.pointId, params.deliveryId),
	component: function DeliveryCorrectionRoute() {
		const { pointId, deliveryId } = Route.useParams();

		return <DeliveryCorrectionPage pointId={pointId} deliveryId={deliveryId} />;
	},
});
