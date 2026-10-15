import { Alert, Checkbox, FormControlLabel, TextField } from '@mui/material';
import { useState } from 'react';

import {
	unknownUserEmail,
	type Company,
	type Point,
} from '@entities/organization';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { FormDialog } from '@shared/ui/form-dialog';

import {
	useCreatePoint,
	useDisableCompany,
	useTransferCompany,
} from '../api/company-writes';

export const transferWarning =
	'Новый владелец сразу получит все права. Вы потеряете доступ к компании, её точкам и каталогу.';

export const disableWarning =
	'Все точки компании перестанут работать, сотрудники потеряют доступ. Данные останутся для чтения. Включить компанию обратно нельзя.';

interface NewPointDialogProps {
	company: Company;
	onCreated: (point: Point) => void;
	onClose: () => void;
}

/** Новая точка в компании: обязательное название. */
export function NewPointDialog({
	company,
	onCreated,
	onClose,
}: NewPointDialogProps) {
	const [name, setName] = useState('');
	const [tried, setTried] = useState(false);
	const mutation = useCreatePoint(company.id);
	const value = name.trim();
	const fieldError =
		tried && !value ? 'Введите название точки' : mutation.error?.message;

	return (
		<FormDialog
			title={`Новая точка в компании «${company.name}»`}
			submitLabel='Создать точку'
			pending={mutation.isPending}
			onSubmit={() => {
				setTried(true);

				if (value) {
					mutation.mutate(value, { onSuccess: onCreated });
				}
			}}
			onClose={onClose}
		>
			<TextField
				label='Название точки'
				placeholder='Например, Центр'
				autoFocus
				value={name}
				onChange={(event) => setName(event.target.value)}
				error={!!fieldError}
				helperText={
					fieldError ||
					'Точка — место со своим складом и кухней. Каталог у точек компании общий.'
				}
			/>
		</FormDialog>
	);
}

interface CompanyDialogProps {
	company: Company;
	onDone: () => void;
	onClose: () => void;
}

/** Передача компании по почте с отметкой согласия. */
export function TransferDialog({
	company,
	onDone,
	onClose,
}: CompanyDialogProps) {
	const [email, setEmail] = useState('');
	const [agreed, setAgreed] = useState(false);
	const mutation = useTransferCompany(company.id);
	const unknownEmail = unknownUserEmail(mutation.error, mutation.variables);

	return (
		<FormDialog
			title={`Передать компанию «${company.name}»`}
			submitLabel='Передать компанию'
			submitColor='error'
			submitDisabled={!agreed || !email.trim()}
			pending={mutation.isPending}
			error={unknownEmail ? undefined : mutation.error?.message}
			onSubmit={() => mutation.mutate(email.trim(), { onSuccess: onDone })}
			onClose={onClose}
		>
			<TextField
				label='Почта нового владельца'
				type='email'
				autoFocus
				value={email}
				onChange={(event) => setEmail(event.target.value)}
				error={!!unknownEmail}
				helperText={
					unknownEmail
						? `С почтой ${unknownEmail} в Goden ещё никто не входил. Получатель должен сначала войти.`
						: 'Он должен хотя бы раз войти в Goden'
				}
			/>
			<Alert severity='error' icon={false}>
				{transferWarning} Вернуть компанию может только новый владелец.
			</Alert>
			<FormControlLabel
				control={
					<Checkbox
						checked={agreed}
						onChange={(event) => setAgreed(event.target.checked)}
					/>
				}
				label='Понимаю, что потеряю доступ'
			/>
		</FormDialog>
	);
}

/** Отключение компании с подтверждением вводом её названия. */
export function DisableCompanyDialog({
	company,
	onDone,
	onClose,
}: CompanyDialogProps) {
	const [typed, setTyped] = useState('');
	const mutation = useDisableCompany(company.id);

	return (
		<ConfirmDialog
			open
			title={`Отключить компанию «${company.name}»?`}
			description={disableWarning}
			confirmLabel='Отключить компанию'
			confirmColor='error'
			confirmDisabled={typed.trim() !== company.name}
			pending={mutation.isPending}
			error={mutation.error?.message}
			onConfirm={() => mutation.mutate(undefined, { onSuccess: onDone })}
			onClose={onClose}
		>
			<TextField
				label={`Введите «${company.name}», чтобы подтвердить`}
				fullWidth
				value={typed}
				onChange={(event) => setTyped(event.target.value)}
			/>
		</ConfirmDialog>
	);
}
