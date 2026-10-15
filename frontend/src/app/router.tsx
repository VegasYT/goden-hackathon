import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRouter } from '@tanstack/react-router';
import type { ReactNode } from 'react';

import type { Session } from '@shared/auth';

import { AppErrorComponent } from './app-error-component';
import { routeTree } from './routeTree.gen';

/** Контекст корневого роута: клиент запросов и сессия вошедшего пользователя. */
export interface RouterContext {
	queryClient: QueryClient;
	session: Session;
}

/** Настройки роутера, общие для приложения и `renderApp` в тестах. */
export function appRouterOptions(context: RouterContext) {
	return {
		routeTree,
		context,
		defaultErrorComponent: AppErrorComponent,
		defaultPreload: 'intent',
		// Свежестью данных при предзагрузке управляет TanStack Query, а не кэш роутера.
		defaultPreloadStaleTime: 0,
		Wrap: ({ children }: { children: ReactNode }) => (
			<QueryClientProvider client={context.queryClient}>
				{children}
			</QueryClientProvider>
		),
	} as const;
}

/** Создаёт роутер приложения для вошедшего пользователя. */
export function createAppRouter(context: RouterContext) {
	return createRouter(appRouterOptions(context));
}

declare module '@tanstack/react-router' {
	interface Register {
		router: ReturnType<typeof createAppRouter>;
	}

	interface HistoryState {
		/** Поставку только что зарегистрировали: карточка показывает об этом сообщение. */
		registeredDelivery?: { number: string; positions: number };
	}
}
