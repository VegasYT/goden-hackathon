import { Box, Stack } from '@mui/material';
import type { ReactNode } from 'react';

import { coarsePointer } from '@shared/ui/theme';

export interface ActionBarProps {
	/** Итог слева, например прогресс и разбивка по исходам. */
	children?: ReactNode;
	action: ReactNode;
	/** Подпись для программ чтения с экрана. */
	label?: string;
}

/** Панель итога и основного действия внизу экрана; ставится последним элементом страницы. */
export function ActionBar({
	children,
	action,
	label = 'Итог и действие',
}: ActionBarProps) {
	return (
		<Box
			component='section'
			aria-label={label}
			sx={{
				// Sticky в конце потока, поэтому панель не перекрывает последний элемент списка.
				position: 'sticky',
				bottom: 0,
				// На короткой странице панель прижимается к низу окна, а не висит под содержимым.
				mt: 'auto',
				zIndex: 'appBar',
				display: 'flex',
				alignItems: 'center',
				gap: 3,
				px: 4,
				py: 1.75,
				bgcolor: 'background.paper',
				borderTop: '1px solid',
				borderColor: 'divider',
				[coarsePointer]: { px: 3 },
			}}
		>
			<Box sx={{ flex: 1, minWidth: 0 }}>{children}</Box>
			<Stack
				direction='row'
				sx={{ flexShrink: 0, gap: 1, whiteSpace: 'nowrap' }}
			>
				{action}
			</Stack>
		</Box>
	);
}
