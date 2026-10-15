import {
	Alert,
	Box,
	Button,
	Paper,
	TextField,
	Typography,
} from '@mui/material';
import { useMutation } from '@tanstack/react-query';
import { useId, useState, type FormEvent } from 'react';

import { ApiError } from '@shared/api';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { Icon } from '@shared/ui/icon';

interface NameStepProps {
	step: 1 | 2;
	done?: string;
	title: string;
	description: string;
	label: string;
	placeholder?: string;
	hint?: string;
	emptyError: string;
	submitLabel: string;
	/** Отправляет название; ключ идемпотентности один на попытку, пока название не меняли. */
	onSubmit: (name: string, idempotencyKey: string) => Promise<unknown>;
}

/** Карточка шага первого входа с одним обязательным полем названия. */
export function NameStep({
	step,
	done,
	title,
	description,
	label,
	placeholder,
	hint,
	emptyError,
	submitLabel,
	onSubmit,
}: NameStepProps) {
	const titleId = useId();
	const [name, setName] = useState('');
	const [emptyTried, setEmptyTried] = useState(false);
	const keyFor = useIdempotencyKey();
	const mutation = useMutation({
		mutationFn: ({ value, key }: { value: string; key: string }) =>
			onSubmit(value, key),
	});
	const serverError =
		mutation.error instanceof ApiError ? mutation.error : null;
	const fieldError = emptyTried
		? emptyError
		: (serverError?.fieldErrors.name?.message ?? undefined);

	const submit = (event: FormEvent) => {
		event.preventDefault();

		const value = name.trim();

		if (!value) {
			setEmptyTried(true);
			return;
		}

		mutation.mutate({ value, key: keyFor({ name: value }) });
	};

	return (
		<Paper
			component='section'
			aria-labelledby={titleId}
			sx={{ width: '100%', maxWidth: 520, p: 4 }}
		>
			<Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, mb: 2.5 }}>
				<Box aria-hidden sx={{ display: 'flex', gap: 0.5 }}>
					{[1, 2].map((index) => (
						<Box
							key={index}
							sx={{
								width: 32,
								height: 6,
								borderRadius: 1,
								bgcolor: index <= step ? 'primary.main' : 'track',
							}}
						/>
					))}
				</Box>
				<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
					Шаг {step} из 2
				</Typography>
			</Box>
			{done && (
				<Typography
					role='status'
					variant='body2'
					sx={{ display: 'flex', alignItems: 'center', gap: 0.75, mb: 1.25 }}
				>
					<Icon name='done' sx={{ color: 'primary.ink' }} />
					{done}
				</Typography>
			)}
			<Typography id={titleId} variant='h2' component='h1'>
				{title}
			</Typography>
			<Typography sx={{ mt: 1, color: 'text.secondary' }}>
				{description}
			</Typography>
			<Box component='form' noValidate onSubmit={submit} sx={{ mt: 3 }}>
				<TextField
					label={label}
					value={name}
					onChange={(event) => {
						setName(event.target.value);
						setEmptyTried(false);
					}}
					placeholder={placeholder}
					required
					fullWidth
					autoFocus
					error={Boolean(fieldError)}
					helperText={fieldError ?? hint}
					slotProps={{ htmlInput: { maxLength: 200 } }}
				/>
				{serverError && !serverError.fieldErrors.name && (
					<Alert severity='error' sx={{ mt: 2 }}>
						{serverError.message}
					</Alert>
				)}
				<Button
					type='submit'
					variant='contained'
					fullWidth
					loading={mutation.isPending}
					sx={{ mt: 3 }}
				>
					{submitLabel}
				</Button>
			</Box>
		</Paper>
	);
}
