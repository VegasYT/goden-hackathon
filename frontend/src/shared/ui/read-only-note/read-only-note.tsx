import { Alert } from '@mui/material';
import type { ReactNode } from 'react';

import { Icon } from '@shared/ui/icon';

/** Пояснение с замком, почему действия на экране недоступны: точка отключена или нет права. */
export function ReadOnlyNote({ children }: { children: ReactNode }) {
	return (
		<Alert severity='info' role='note' icon={<Icon name='lock' />}>
			{children}
		</Alert>
	);
}
