import { Box, Stack, Typography } from '@mui/material';
import type { ElementType, ReactNode } from 'react';

import { Icon, type IconName } from '@shared/ui/icon';
import { radius } from '@shared/ui/theme';

export interface EmptyStateProps {
	/** Заголовок-приглашение, например «Поставок пока нет». */
	title: string;
	description?: ReactNode;
	/** Действие, которое помогает начать, например кнопка или ссылка. */
	action?: ReactNode;
	icon?: IconName;
	/** Уровень заголовка в структуре страницы. */
	titleComponent?: ElementType;
}

export function EmptyState({
	title,
	description,
	action,
	icon = 'empty',
	titleComponent = 'h2',
}: EmptyStateProps) {
	return (
		<Stack
			sx={{ alignItems: 'center', textAlign: 'center', py: 6, px: 3, gap: 1.5 }}
		>
			<Box
				sx={(theme) => ({
					display: 'grid',
					placeItems: 'center',
					width: 48,
					height: 48,
					borderRadius: radius(theme, 'lg'),
					bgcolor: 'track',
					color: 'text.secondary',
				})}
			>
				<Icon name={icon} size='lg' />
			</Box>
			<Typography variant='h3' component={titleComponent}>
				{title}
			</Typography>
			{description && (
				<Typography sx={{ color: 'text.secondary', maxWidth: 420 }}>
					{description}
				</Typography>
			)}
			{action && <Box sx={{ mt: 1 }}>{action}</Box>}
		</Stack>
	);
}
