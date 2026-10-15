import { Stack, Typography } from '@mui/material';
import { Children, Fragment, type ReactNode } from 'react';

import { Icon } from '@shared/ui/icon';

/** Путь к странице для `PageHeader`: ссылки `BreadcrumbLink` и текущая страница `current`. */
export function Breadcrumbs({
	current,
	children,
}: {
	current: string;
	children: ReactNode;
}) {
	return (
		<Stack direction='row' sx={{ gap: 0.5, alignItems: 'center' }}>
			{Children.map(children, (link) => (
				<Fragment>
					{link}
					<Icon name='chevronRight' size='sm' sx={{ color: 'text.tertiary' }} />
				</Fragment>
			))}
			<Typography
				variant='body2'
				component='span'
				sx={{ color: 'text.secondary' }}
			>
				{current}
			</Typography>
		</Stack>
	);
}
