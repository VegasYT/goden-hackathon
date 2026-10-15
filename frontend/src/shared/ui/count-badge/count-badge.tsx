import { Box, type SxProps, type Theme } from '@mui/material';

import { radius } from '@shared/ui/theme';

/** Небольшое число рядом с подписью, например «Открытый карантин 4»; `tone='warning'` — то, что требует внимания. */
export function CountBadge({
	count,
	tone = 'neutral',
	sx,
}: {
	count: number;
	tone?: 'neutral' | 'warning';
	sx?: SxProps<Theme>;
}) {
	return (
		<Box
			component='span'
			sx={[
				(theme) => ({
					px: 0.875,
					borderRadius: radius(theme, 'xs'),
					bgcolor: tone === 'warning' ? 'warning.tint' : 'track',
					color: tone === 'warning' ? 'warning.ink' : 'text.primary',
					typography: 'body2',
					fontWeight: 600,
				}),
				...(Array.isArray(sx) ? sx : [sx]),
			]}
		>
			{count}
		</Box>
	);
}
