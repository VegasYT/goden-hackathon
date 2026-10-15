import type { ReactNode } from 'react';

import type { Session } from './session';
import { SessionContext } from './session-context';

/** Отдаёт сессию вошедшего пользователя в `useSession` для всего дерева внутри. */
export function SessionProvider({
	session,
	children,
}: {
	session: Session;
	children: ReactNode;
}) {
	return <SessionContext value={session}>{children}</SessionContext>;
}
