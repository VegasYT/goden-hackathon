import {
	Alert,
	Button,
	Dialog,
	DialogActions,
	DialogContent,
	DialogTitle,
	Stack,
	type DialogProps,
} from '@mui/material';
import type { FormEvent, ReactNode } from 'react';

interface FormDialogProps {
	title: string;
	submitLabel: string;
	/** `error` — для необратимого действия. */
	submitColor?: 'primary' | 'error';
	/** Отправка недоступна, пока не выполнено условие в форме. */
	submitDisabled?: boolean;
	/** Запрос выполняется: кнопка показывает индикатор, диалог не закрывается. */
	pending?: boolean;
	/** Ошибка запроса, которая не относится к полю; показывается над кнопками. */
	error?: ReactNode;
	maxWidth?: DialogProps['maxWidth'];
	/** Отправка формы кнопкой или Enter; проверку полей делает вызывающий. */
	onSubmit: () => void;
	onClose: () => void;
	children: ReactNode;
}

/** Диалог с формой: заголовок, поля, «Отмена» и кнопка отправки. */
export function FormDialog({
	title,
	submitLabel,
	submitColor = 'primary',
	submitDisabled = false,
	pending = false,
	error,
	maxWidth = 'xs',
	onSubmit,
	onClose,
	children,
}: FormDialogProps) {
	const submit = (event: FormEvent) => {
		event.preventDefault();

		if (!submitDisabled && !pending) {
			onSubmit();
		}
	};

	return (
		<Dialog
			open
			onClose={pending ? undefined : onClose}
			maxWidth={maxWidth}
			fullWidth
		>
			<form noValidate onSubmit={submit}>
				<DialogTitle>{title}</DialogTitle>
				<DialogContent>
					<Stack sx={{ gap: 2.5, pt: 0.5 }}>
						{children}
						{error && <Alert severity='error'>{error}</Alert>}
					</Stack>
				</DialogContent>
				<DialogActions>
					<Button variant='outlined' onClick={onClose} disabled={pending}>
						Отмена
					</Button>
					<Button
						type='submit'
						variant='contained'
						color={submitColor}
						loading={pending}
						disabled={submitDisabled}
					>
						{submitLabel}
					</Button>
				</DialogActions>
			</form>
		</Dialog>
	);
}
