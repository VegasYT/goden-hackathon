import { Box, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';

import { Icon, type IconName } from '@shared/ui/icon';

interface TimelineItemProps {
	icon: IconName;
	/** Цвет значка и его подложки; по умолчанию нейтральные. */
	tone?: { color: string; bg: string };
	children: ReactNode;
	sx?: SxProps<Theme>;
}

const neutral = { color: 'text.secondary', bg: 'track' };

/** Событие ленты истории: круглый значок слева, содержимое справа. */
export function TimelineItem({
	icon,
	tone = neutral,
	children,
	sx,
}: TimelineItemProps) {
	return (
		<Box
			component='li'
			sx={[
				{
					display: 'grid',
					gridTemplateColumns: '32px minmax(0, 1fr)',
					gap: 1.5,
				},
				...(Array.isArray(sx) ? sx : [sx]),
			]}
		>
			<Box
				aria-hidden
				sx={{
					width: 32,
					height: 32,
					display: 'grid',
					placeItems: 'center',
					borderRadius: '50%',
					bgcolor: tone.bg,
					color: tone.color,
				}}
			>
				<Icon name={icon} size='sm' />
			</Box>
			<Box sx={{ minWidth: 0 }}>{children}</Box>
		</Box>
	);
}
