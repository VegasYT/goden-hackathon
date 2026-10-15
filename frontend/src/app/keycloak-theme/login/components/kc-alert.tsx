import { Alert } from '@mui/material';
import type { ReactNode } from 'react';

export type KcMessageType = 'success' | 'warning' | 'error' | 'info';

/** Сообщение Keycloak над формой: цвет по смыслу, информационное без статусного цвета. */
export function KcAlert({
	type,
	id,
	icon,
	children,
}: {
	type: KcMessageType;
	id?: string;
	icon?: ReactNode;
	children: ReactNode;
}) {
	const urgent = type === 'error' || type === 'warning';

	return (
		<Alert
			id={id}
			severity={type}
			icon={icon}
			role={urgent ? 'alert' : 'status'}
		>
			{children}
		</Alert>
	);
}
