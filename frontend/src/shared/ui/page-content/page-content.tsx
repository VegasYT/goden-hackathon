import { Box } from '@mui/material';
import type { ReactNode } from 'react';

/** Контейнер содержимого страницы со стандартными отступами; `actionBar` встаёт после него, внизу страницы. */
export function PageContent({
	children,
	actionBar,
}: {
	children: ReactNode;
	actionBar?: ReactNode;
}) {
	return (
		<>
			<Box sx={{ px: { xs: 3, lg: 4 }, py: 2.5 }}>{children}</Box>
			{actionBar}
		</>
	);
}
