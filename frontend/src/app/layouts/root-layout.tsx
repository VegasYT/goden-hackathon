import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { Outlet } from '@tanstack/react-router';
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools';

export function RootLayout() {
	return (
		<>
			<Outlet />
			<ReactQueryDevtools buttonPosition='bottom-right' />
			<TanStackRouterDevtools position='top-right' />
		</>
	);
}
