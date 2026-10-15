import { Box, Drawer, IconButton, Stack, Typography } from '@mui/material';
import { useId, type ReactNode } from 'react';

import { CountBadge } from '@shared/ui/count-badge';
import { Icon } from '@shared/ui/icon';
import { useWideLayout } from '@shared/ui/theme';

export interface SidePanelProps {
	open: boolean;
	title: string;
	/** Число рядом с заголовком, например позиций в списке панели. */
	count?: number;
	/** Подпись кнопки закрытия: «Закрыть историю». */
	closeLabel: string;
	onClose: () => void;
	/** Ширина справа на широком экране; уже `lg` панель занимает весь экран. */
	width?: number;
	children: ReactNode;
}

/** Боковая панель поверх страницы: справа на широком экране, на весь экран на планшете. */
export function SidePanel({
	open,
	title,
	count,
	closeLabel,
	onClose,
	width = 480,
	children,
}: SidePanelProps) {
	const wide = useWideLayout();
	const titleId = useId();

	return (
		<Drawer
			anchor='right'
			open={open}
			onClose={onClose}
			slotProps={{
				paper: {
					role: 'dialog',
					'aria-modal': true,
					'aria-labelledby': titleId,
					sx: { width: wide ? width : '100%', maxWidth: '100%' },
				},
			}}
		>
			<Stack
				direction='row'
				sx={{
					alignItems: 'center',
					gap: 1,
					pl: wide ? 3 : 1.5,
					pr: wide ? 1.5 : 3,
					py: 1.5,
					borderBottom: '1px solid',
					borderColor: 'divider',
					flexDirection: wide ? 'row' : 'row-reverse',
				}}
			>
				<Stack
					direction='row'
					sx={{ flex: 1, minWidth: 0, gap: 1.25, alignItems: 'center' }}
				>
					<Typography id={titleId} variant='h2' noWrap>
						{title}
					</Typography>
					{count !== undefined && <CountBadge count={count} />}
				</Stack>
				<IconButton aria-label={closeLabel} onClick={onClose}>
					<Icon name='close' />
				</IconButton>
			</Stack>
			<Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>{children}</Box>
		</Drawer>
	);
}
