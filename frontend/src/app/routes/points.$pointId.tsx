import { createFileRoute } from '@tanstack/react-router';

import { AppErrorComponent } from '../app-error-component';
import { PointLayout, PointUnavailable } from '../layouts/point-layout';
import { PointUnavailableError, requirePoint } from '../point-access';

export const Route = createFileRoute('/points/$pointId')({
	beforeLoad: async ({ context, params }) => ({
		point: await requirePoint(context.queryClient, params.pointId),
	}),
	component: function PointRoute() {
		return <PointLayout pointId={Route.useParams().pointId} />;
	},
	errorComponent: (props) =>
		props.error instanceof PointUnavailableError ? (
			<PointUnavailable />
		) : (
			<AppErrorComponent {...props} />
		),
});
