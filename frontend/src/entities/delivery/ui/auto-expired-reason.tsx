import { Box, Typography } from '@mui/material';

import { reasonLabel } from '../model/reason';

/** Причина отклонения просроченной позиции вместо выбора причины: её проставляет система. */
export function AutoExpiredReason() {
	return (
		<Box>
			<Typography variant='body2' sx={{ color: 'text.strong' }}>
				{reasonLabel('expired')}
			</Typography>
			<Typography variant='caption' sx={{ color: 'text.secondary' }}>
				проставлена автоматически
			</Typography>
		</Box>
	);
}
