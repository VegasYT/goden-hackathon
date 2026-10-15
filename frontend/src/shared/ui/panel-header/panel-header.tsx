import { Stack } from '@mui/material';
import type { ReactNode } from 'react';

/** Шапка панели со списком: заголовок или фильтры слева, действия справа, разделитель снизу. */
export function PanelHeader({ children }: { children: ReactNode }) {
	return (
		<Stack
			direction='row'
			sx={{
				alignItems: 'center',
				justifyContent: 'space-between',
				flexWrap: 'wrap',
				gap: 1.5,
				px: 2,
				py: 1.5,
				borderBottom: '1px solid',
				borderColor: 'divider',
			}}
		>
			{children}
		</Stack>
	);
}
