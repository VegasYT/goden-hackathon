import { Card, Stack, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';

/** Список карточек на планшете: `label` называет список, карточки — `CardListItem`. */
export function CardList({
	label,
	children,
	sx,
}: {
	label: string;
	children: ReactNode;
	sx?: SxProps<Theme>;
}) {
	return (
		<Stack
			component='ul'
			aria-label={label}
			sx={[
				{ listStyle: 'none', m: 0, p: 2, gap: 1 },
				...(Array.isArray(sx) ? sx : [sx]),
			]}
		>
			{children}
		</Stack>
	);
}

/** Карточка списка: строка с содержимым слева и действиями справа; другую раскладку задаёт `sx`. */
export function CardListItem({
	children,
	label,
	sx,
}: {
	children: ReactNode;
	/** Название карточки для программ чтения с экрана, если его нет в начале текста. */
	label?: string;
	sx?: SxProps<Theme>;
}) {
	return (
		<Card
			component='li'
			aria-label={label}
			sx={[
				{
					display: 'flex',
					alignItems: 'center',
					gap: 1.5,
					p: 1.5,
					pl: 2,
				},
				...(Array.isArray(sx) ? sx : [sx]),
			]}
		>
			{children}
		</Card>
	);
}
