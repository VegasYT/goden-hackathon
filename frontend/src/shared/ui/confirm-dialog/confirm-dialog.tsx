import {
	Alert,
	Box,
	Button,
	Checkbox,
	Dialog,
	DialogActions,
	DialogContent,
	DialogTitle,
	FormControlLabel,
	Stack,
	Typography,
} from '@mui/material';
import { useId, useState, type ReactNode } from 'react';

import { readStorage, writeStorage } from '@shared/lib/storage';
import { Icon } from '@shared/ui/icon';
import { radius, themeVars } from '@shared/ui/theme';

export interface DismissibleNotice {
	/** Ключ, под которым запоминается «больше не показывать». */
	id: string;
	text: string;
}

export interface ConfirmDialogProps {
	open: boolean;
	title: string;
	/** Что произойдёт после подтверждения. */
	description?: ReactNode;
	/** Итоги действия, например разбивка позиций приёмки. */
	children?: ReactNode;
	confirmLabel: string;
	cancelLabel?: string;
	/** `error` — для необратимого удаления или отклонения. */
	confirmColor?: 'primary' | 'error';
	onConfirm: () => void;
	/** Закрытие без действия: «Отмена», Escape или клик по фону. */
	onClose: () => void;
	/** Предупреждение, которое можно скрыть; само подтверждение показывается всегда. */
	dismissibleNotice?: DismissibleNotice;
	/** Действие выполняется: кнопка показывает индикатор, диалог не закрывается и не подтверждается повторно. */
	pending?: boolean;
	/** Ошибка действия, показывается над кнопками. */
	error?: ReactNode;
	/** Подтверждение недоступно, пока не выполнено условие в содержимом диалога. */
	confirmDisabled?: boolean;
}

const noticeKey = (id: string) => `goden.notice.${id}`;

/**
 * Предупреждение с отметкой «Больше не показывать». Скрыто ли оно, читается один раз:
 * Dialog монтирует содержимое при каждом открытии, и блок остаётся на месте, пока идёт подтверждение и после ошибки.
 */
function NoticeBox({
	notice,
	checked,
	onCheckedChange,
}: {
	notice: DismissibleNotice;
	checked: boolean;
	onCheckedChange: (checked: boolean) => void;
}) {
	const [visible] = useState(
		() => readStorage(noticeKey(notice.id)) !== 'hidden'
	);

	if (!visible) {
		return null;
	}

	return (
		<Box
			sx={(theme) => ({
				mt: 2,
				px: 1.75,
				py: 1.5,
				borderRadius: radius(theme, 'md'),
				bgcolor: 'background.default',
				boxShadow: `inset 0 0 0 1px ${themeVars(theme).palette.divider}`,
			})}
		>
			<Stack direction='row' sx={{ gap: 1.25, alignItems: 'flex-start' }}>
				<Icon name='info' sx={{ color: 'text.secondary', mt: 0.25 }} />
				<Typography>{notice.text}</Typography>
			</Stack>
			<FormControlLabel
				sx={{ ml: 3, mt: 0.5 }}
				control={
					<Checkbox
						size='small'
						checked={checked}
						onChange={(event) => onCheckedChange(event.target.checked)}
					/>
				}
				label={
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						Больше не показывать это предупреждение
					</Typography>
				}
			/>
		</Box>
	);
}

export function ConfirmDialog({
	open,
	title,
	description,
	children,
	confirmLabel,
	cancelLabel = 'Отмена',
	confirmColor = 'primary',
	onConfirm,
	onClose,
	dismissibleNotice,
	pending = false,
	error,
	confirmDisabled = false,
}: ConfirmDialogProps) {
	const descriptionId = useId();
	const [dontShowAgain, setDontShowAgain] = useState(false);

	const confirm = () => {
		if (dismissibleNotice && dontShowAgain) {
			writeStorage(noticeKey(dismissibleNotice.id), 'hidden');
		}

		onConfirm();
	};

	return (
		<Dialog
			open={open}
			onClose={pending ? undefined : onClose}
			fullWidth
			maxWidth='sm'
			aria-describedby={description ? descriptionId : undefined}
			slotProps={{ paper: { role: 'alertdialog' } }}
		>
			<DialogTitle>{title}</DialogTitle>
			<DialogContent>
				{description && (
					<Typography
						id={descriptionId}
						sx={{ color: 'text.secondary', mb: 1.5 }}
					>
						{description}
					</Typography>
				)}
				{children}
				{dismissibleNotice && (
					<NoticeBox
						notice={dismissibleNotice}
						checked={dontShowAgain}
						onCheckedChange={setDontShowAgain}
					/>
				)}
				{error && (
					<Alert severity='error' sx={{ mt: 2 }}>
						{error}
					</Alert>
				)}
			</DialogContent>
			<DialogActions>
				<Button variant='outlined' onClick={onClose} disabled={pending}>
					{cancelLabel}
				</Button>
				<Button
					variant='contained'
					color={confirmColor}
					onClick={confirm}
					loading={pending}
					disabled={confirmDisabled}
				>
					{confirmLabel}
				</Button>
			</DialogActions>
		</Dialog>
	);
}
