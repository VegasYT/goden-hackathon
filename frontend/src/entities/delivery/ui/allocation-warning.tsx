import { Typography } from '@mui/material';

import { Icon } from '@shared/ui/icon';

/** Предупреждение под полями решения, например что количество распределено не полностью. */
export function AllocationWarning({ children }: { children: string }) {
	return (
		<Typography
			variant='body2'
			sx={{
				color: 'warning.ink',
				display: 'flex',
				gap: 0.75,
				alignItems: 'center',
			}}
		>
			<Icon name='warning' size='sm' />
			{children}
		</Typography>
	);
}
