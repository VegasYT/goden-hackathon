import { queryOptions, type QueryClient } from '@tanstack/react-query';

import { getData, type components } from '@shared/api';

export type Profile = components['schemas']['Profile'];
export type ProfilePoint = components['schemas']['ProfilePoint'];

/** Профиль вошедшего пользователя: его компании и точки с правами на каждой. */
export const meQueryOptions = queryOptions({
	queryKey: ['me'],
	queryFn: getData<Profile>('/me'),
	// Профиль перечитывается при возврате на вкладку и по запросу, а не при открытии каждой страницы.
	staleTime: 0,
	refetchOnMount: false,
});

/** Перечитывает профиль после записи организации и возвращает новый. */
export function refreshProfile(queryClient: QueryClient): Promise<Profile> {
	return queryClient.fetchQuery(meQueryOptions);
}
