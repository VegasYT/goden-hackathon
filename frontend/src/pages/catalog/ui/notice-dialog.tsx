import {
	Button,
	Dialog,
	DialogActions,
	DialogContent,
	DialogTitle,
} from '@mui/material';
import type { ReactNode } from 'react';

/** Диалог-отказ: почему действие нельзя выполнить, и одна кнопка «Понятно». */
export function NoticeDialog({
	title,
	children,
	onClose,
}: {
	title: string;
	children: ReactNode;
	onClose: () => void;
}) {
	return (
		<Dialog open onClose={onClose} maxWidth='xs' fullWidth>
			<DialogTitle>{title}</DialogTitle>
			<DialogContent>{children}</DialogContent>
			<DialogActions>
				<Button variant='outlined' onClick={onClose}>
					Понятно
				</Button>
			</DialogActions>
		</Dialog>
	);
}
