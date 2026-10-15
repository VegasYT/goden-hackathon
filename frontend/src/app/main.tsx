import '@fontsource-variable/golos-text';
import type { QueryClient } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { StrictMode, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';

import { meQueryOptions } from '@entities/profile';
import { AuthUnavailablePage } from '@pages/errors/auth-unavailable';
import { ProfileUnavailablePage } from '@pages/errors/profile-unavailable';
import { SessionProvider, startSession, type Session } from '@shared/auth';
import { ThemeRoot } from '@shared/ui/theme';

import { KcPage } from './keycloak-theme/kc.gen';
import { createQueryClient } from './query-client';
import { createAppRouter } from './router';
import { StartupScreen } from './startup-screen';

const root = createRoot(document.getElementById('root')!);

function render(node: ReactNode) {
	root.render(
		<StrictMode>
			<ThemeRoot>{node}</ThemeRoot>
		</StrictMode>
	);
}

async function openApp(session: Session, queryClient: QueryClient) {
	render(<StartupScreen />);

	try {
		await queryClient.fetchQuery(meQueryOptions);
	} catch (error) {
		render(
			<ProfileUnavailablePage
				error={error}
				onRetry={() => void openApp(session, queryClient)}
			/>
		);
		return;
	}

	const router = createAppRouter({ queryClient, session });

	render(
		<SessionProvider session={session}>
			<RouterProvider router={router} />
		</SessionProvider>
	);
}

async function startApp() {
	render(<StartupScreen />);

	const result = await startSession();

	switch (result.status) {
		case 'unavailable':
			render(<AuthUnavailablePage />);
			break;

		case 'signed-in':
			await openApp(result.session, createQueryClient());
			break;

		case 'redirecting':
			break;
	}
}

// Только в dev-сервере: `?kcPage=<pageId>` открывает страницу входа Keycloak с тестовыми данными.
if (
	import.meta.env.DEV &&
	new URLSearchParams(window.location.search).has('kcPage')
) {
	const { kcContextFromSearch } = await import('./keycloak-theme/login/mocks');

	window.kcContext = kcContextFromSearch(window.location.search);
}

// Keycloak отдаёт ту же сборку со своими данными страницы: тогда рисуется страница входа, а не приложение.
if (window.kcContext) {
	render(<KcPage kcContext={window.kcContext} />);
} else {
	await startApp();
}
