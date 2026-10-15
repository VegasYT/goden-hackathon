import { Box, CircularProgress } from '@mui/material';
import { useEffect, useState } from 'react';

import { StandaloneScreen } from '@shared/ui/standalone-screen';

const indicatorDelayMs = 400;

/** Экран на время подъёма сессии до рендера приложения. */
export function StartupScreen() {
	const [slow, setSlow] = useState(false);

	useEffect(() => {
		const timer = setTimeout(() => setSlow(true), indicatorDelayMs);

		return () => clearTimeout(timer);
	}, []);

	if (!slow) {
		return <Box sx={{ minHeight: '100dvh', bgcolor: 'background.default' }} />;
	}

	return (
		<StandaloneScreen>
			<CircularProgress aria-label='Вход в приложение' />
		</StandaloneScreen>
	);
}
