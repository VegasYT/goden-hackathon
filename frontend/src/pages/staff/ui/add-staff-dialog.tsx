import {
	FormControl,
	FormControlLabel,
	FormLabel,
	Radio,
	RadioGroup,
	TextField,
	Typography,
} from '@mui/material';
import { useId, useState } from 'react';

import { roleChoices, unknownUserEmail } from '@entities/organization';
import { roleLabels, type Role } from '@shared/auth';
import { FormDialog } from '@shared/ui/form-dialog';

import { useAssignStaff } from '../api/staff-writes';

const roleHints: Record<Role, string> = {
	manager: 'Ведёт точку: склад, поставки, меню и сотрудники',
	approver: 'Принимает поставки, видит склад',
	cook: 'Кухня и стоп-лист',
	waiter: 'Заказы и горячие предложения',
};

interface AddStaffDialogProps {
	pointId: string;
	/** Владелец назначает и управляющих. */
	owner: boolean;
	onClose: () => void;
}

/** Добавление сотрудника на точку по почте, с которой он входит в Goden. */
export function AddStaffDialog({
	pointId,
	owner,
	onClose,
}: AddStaffDialogProps) {
	const roleLabelId = useId();
	const [email, setEmail] = useState('');
	const [role, setRole] = useState<Role | null>(null);
	const [tried, setTried] = useState(false);
	const mutation = useAssignStaff(pointId);
	const unknownEmail = unknownUserEmail(
		mutation.error,
		mutation.variables?.email
	);
	const emailError =
		tried && !email.trim()
			? 'Введите почту сотрудника'
			: unknownEmail &&
				`Пользователь с почтой ${unknownEmail} ещё не входил в Goden. Попросите его зарегистрироваться и войти, затем добавьте снова.`;
	const roleError = tried && !role ? 'Выберите роль' : undefined;

	const submit = () => {
		setTried(true);

		const value = email.trim();

		if (value && role) {
			mutation.mutate({ email: value, role }, { onSuccess: onClose });
		}
	};

	return (
		<FormDialog
			title='Добавить сотрудника на точку'
			submitLabel='Добавить'
			pending={mutation.isPending}
			error={unknownEmail ? undefined : mutation.error?.message}
			onSubmit={submit}
			onClose={onClose}
		>
			<TextField
				label='Почта сотрудника'
				type='email'
				autoComplete='off'
				autoFocus
				value={email}
				onChange={(event) => setEmail(event.target.value)}
				error={!!emailError}
				helperText={emailError || 'Почта, с которой сотрудник входит в Goden'}
			/>
			<FormControl error={!!roleError}>
				<FormLabel id={roleLabelId}>Роль</FormLabel>
				<RadioGroup
					aria-labelledby={roleLabelId}
					value={role ?? ''}
					onChange={(event) => setRole(event.target.value as Role)}
				>
					{roleChoices(owner).map(({ role: item, assignable }) => (
						<FormControlLabel
							key={item}
							value={item}
							disabled={!assignable}
							control={<Radio />}
							label={
								<span>
									{roleLabels[item]}
									<Typography
										variant='body2'
										component='span'
										sx={{ display: 'block', color: 'text.secondary' }}
									>
										{assignable ? roleHints[item] : 'Назначает владелец'}
									</Typography>
								</span>
							}
							sx={{
								alignItems: 'flex-start',
								py: 0.5,
								'& .MuiRadio-root': { mt: -0.75 },
							}}
						/>
					))}
				</RadioGroup>
				{roleError && (
					<Typography variant='body2' sx={{ color: 'error.main' }}>
						{roleError}
					</Typography>
				)}
			</FormControl>
		</FormDialog>
	);
}
