import { Typography, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';

import { Icon } from '@shared/ui/icon';

/** Спокойное пояснение с иконкой «i» над списком или под ним. */
export function InfoNote({
	children,
	sx,
}: {
	children: ReactNode;
	sx?: SxProps<Theme>;
}) {
	return (
		<Typography
			variant='body2'
			sx={[
				{
					display: 'flex',
					gap: 1,
					alignItems: 'flex-start',
					color: 'text.secondary',
				},
				...(Array.isArray(sx) ? sx : [sx]),
			]}
		>
			<Icon name='info' size='sm' sx={{ mt: 0.25, flexShrink: 0 }} />
			<span>{children}</span>
		</Typography>
	);
}
