import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import {
	DeliveryPage,
	deliverySearchDefaults,
	deliverySearchSchema,
} from '@pages/delivery';

export const Route = createFileRoute(
	'/points/$pointId/deliveries/$deliveryId/'
)({
	validateSearch: deliverySearchSchema,
	search: { middlewares: [stripSearchParams(deliverySearchDefaults)] },
	component: function DeliveryRoute() {
		const { pointId, deliveryId } = Route.useParams();
		const { panel } = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<DeliveryPage
				pointId={pointId}
				deliveryId={deliveryId}
				panel={panel}
				onPanelChange={(next) => void navigate({ search: { panel: next } })}
			/>
		);
	},
});
