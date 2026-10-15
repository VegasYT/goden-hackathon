import { createContext, use } from 'react';

import type { Session } from './session';

export const SessionContext = createContext<Session | null>(null);

/** Текущий вошедший пользователь; доступен под `SessionProvider`. */
export function useSession(): Session {
	const session = use(SessionContext);

	if (!session) {
		throw new Error('useSession вызван вне SessionProvider');
	}

	return session;
}
