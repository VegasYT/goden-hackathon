import { Box, Stack, Tooltip, Typography } from '@mui/material';

import { formatTime } from '@shared/lib/datetime';

import type { ConnectionState } from './connection-state';

export interface ConnectionIndicatorProps {
	state: ConnectionState;
	/** Только точка с подсказкой — для узкой панели навигации. */
	compact?: boolean;
}

/** Спокойный индикатор связи: «в сети · обновлено 14:32». */
export function ConnectionIndicator({
	state,
	compact = false,
}: ConnectionIndicatorProps) {
	const online = state.status === 'online';
	const text = online
		? `в сети · обновлено ${formatTime(state.updatedAt)}`
		: `нет связи · данные на ${formatTime(state.dataAt)}`;
	const dot = (
		<Box
			component='span'
			sx={{
				width: compact ? 8 : 6,
				height: compact ? 8 : 6,
				borderRadius: '50%',
				flexShrink: 0,
				bgcolor: online ? 'primary.main' : 'warning.main',
			}}
		/>
	);

	if (compact) {
		return (
			<Tooltip title={text}>
				<Box role='status' aria-label={text} sx={{ display: 'flex', p: 1 }}>
					{dot}
				</Box>
			</Tooltip>
		);
	}

	return (
		<Stack role='status' direction='row' sx={{ alignItems: 'center', gap: 1 }}>
			{dot}
			<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
				{text}
			</Typography>
		</Stack>
	);
}
