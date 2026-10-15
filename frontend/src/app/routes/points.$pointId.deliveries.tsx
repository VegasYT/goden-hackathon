import { createFileRoute, Outlet } from '@tanstack/react-router';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/deliveries')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/deliveries'),
	component: Outlet,
});
