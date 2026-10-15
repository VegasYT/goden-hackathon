import { Box, Stack } from '@mui/material';
import type { ReactNode } from 'react';

import { Logo } from '@shared/ui/logo';

/** Экран без оболочки приложения: знак сверху и содержимое по центру. */
export function StandaloneScreen({ children }: { children: ReactNode }) {
	return (
		<Stack
			sx={{
				minHeight: '100dvh',
				bgcolor: 'background.default',
				alignItems: 'center',
				px: 2,
				py: 4,
			}}
		>
			<Logo />
			<Box
				component='main'
				sx={{ flex: 1, display: 'grid', placeItems: 'center', width: '100%' }}
			>
				{children}
			</Box>
		</Stack>
	);
}
