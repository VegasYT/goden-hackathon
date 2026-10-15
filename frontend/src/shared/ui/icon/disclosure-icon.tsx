import type { SxProps, Theme } from '@mui/material';

import { Icon } from './icon';

/** Значок раскрытия строки: «›» у закрытой, «⌄» у раскрытой. */
export function DisclosureIcon({
	open,
	sx,
}: {
	open: boolean;
	sx?: SxProps<Theme>;
}) {
	return (
		<Icon
			name='chevronRight'
			sx={[
				(theme) => ({
					color: 'text.secondary',
					flexShrink: 0,
					transform: open ? 'rotate(90deg)' : undefined,
					transition: theme.transitions.create('transform', {
						duration: theme.transitions.duration.shortest,
					}),
				}),
				...(Array.isArray(sx) ? sx : [sx]),
			]}
		/>
	);
}
