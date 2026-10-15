import { Box, Button, Typography } from '@mui/material';

import { formatTime } from '@shared/lib/datetime';
import { Icon } from '@shared/ui/icon';

import type { ConnectionState } from './connection-state';

export interface ConnectionBannerProps {
	state: Extract<ConnectionState, { status: 'offline' }>;
}

/** Плашка о потере связи с временем данных и кнопкой «Повторить». */
export function ConnectionBanner({ state }: ConnectionBannerProps) {
	return (
		<Box
			role='alert'
			sx={{
				display: 'flex',
				alignItems: 'center',
				gap: 1.5,
				pl: 3,
				pr: 1.5,
				py: 0.5,
				bgcolor: 'warning.tint',
				color: 'warning.onTint',
			}}
		>
			<Icon name='cloudOff' size='lg' />
			<Typography sx={{ flex: 1, color: 'inherit' }}>
				<Box component='span' sx={{ fontWeight: 600 }}>
					Нет связи с сервером.
				</Box>{' '}
				Данные на {formatTime(state.dataAt)} — переподключаемся…
			</Typography>
			<Button
				onClick={state.onRetry}
				sx={{
					color: 'inherit',
					'&:hover': { bgcolor: 'transparent', textDecoration: 'underline' },
				}}
			>
				Повторить
			</Button>
		</Box>
	);
}
