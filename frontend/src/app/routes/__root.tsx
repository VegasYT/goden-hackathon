import { createRootRouteWithContext } from '@tanstack/react-router';

import { NotFoundPage } from '@pages/errors/not-found';

import { RootLayout } from '../layouts/root-layout';
import type { RouterContext } from '../router';

export const Route = createRootRouteWithContext<RouterContext>()({
	component: RootLayout,
	notFoundComponent: NotFoundPage,
});
