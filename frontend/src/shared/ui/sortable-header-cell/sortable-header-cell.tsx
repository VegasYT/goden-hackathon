import {
	TableCell,
	TableSortLabel,
	type SxProps,
	type Theme,
} from '@mui/material';
import type { ReactNode } from 'react';

interface SortableHeaderCellProps {
	children: ReactNode;
	/** Направление, если список сейчас отсортирован по этой колонке. */
	direction?: 'asc' | 'desc';
	onSort: () => void;
	sx?: SxProps<Theme>;
}

/** Заголовок колонки, по которой сортирует сервер; программам чтения с экрана направление сообщает `aria-sort`. */
export function SortableHeaderCell({
	children,
	direction,
	onSort,
	sx,
}: SortableHeaderCellProps) {
	return (
		<TableCell sx={sx} sortDirection={direction ?? false}>
			<TableSortLabel
				active={direction !== undefined}
				direction={direction ?? 'asc'}
				onClick={onSort}
			>
				{children}
			</TableSortLabel>
		</TableCell>
	);
}
