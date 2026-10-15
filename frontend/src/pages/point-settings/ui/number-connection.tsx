import {
	Alert,
	Button,
	Chip,
	Stack,
	TextField,
	Typography,
} from '@mui/material';
import { useState, type FormEvent, type ReactNode } from 'react';

interface ConnectionStatusProps {
	connected: boolean;
	/** Подпись чипа при подключении: «подключены». */
	on: string;
	off: string;
	/** Куда подключено: «в чат −100…». */
	detail: string;
}

/** Строка статуса подключения: зелёный чип с подписью, куда подключено, или серый чип. */
export function ConnectionStatus({
	connected,
	on,
	off,
	detail,
}: ConnectionStatusProps) {
	return (
		<Stack direction='row' sx={{ alignItems: 'center', gap: 1 }}>
			{connected ? (
				<>
					<Chip color='success' label={on} />
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{detail}
					</Typography>
				</>
			) : (
				<Chip label={off} />
			)}
		</Stack>
	);
}

interface NumberConnectFormProps {
	label: string;
	placeholder: string;
	/** Текст под полем, когда `parse` не принял ввод. */
	invalidText: string;
	/** Номер из поля ввода или `null`, если ввод не подходит. */
	parse: (text: string) => number | null;
	/** Подключённый номер; `null` — не подключено. */
	current: number | null;
	/** Идёт сохранение номера. */
	pending: boolean;
	error?: string;
	onSubmit: (number: number) => void;
	/** Кнопки рядом с «Сохранить», например отключение. */
	children?: ReactNode;
}

/** Форма подключения по номеру: «Подключить», а при подключении «Сохранить», неактивная без изменений. */
export function NumberConnectForm({
	label,
	placeholder,
	invalidText,
	parse,
	current,
	pending,
	error,
	onSubmit,
	children,
}: NumberConnectFormProps) {
	const connected = current !== null;
	const [draft, setDraft] = useState(connected ? String(current) : '');
	const [invalid, setInvalid] = useState(false);

	const submit = (event: FormEvent) => {
		event.preventDefault();

		const parsed = parse(draft);

		if (parsed === null) {
			setInvalid(true);
			return;
		}

		onSubmit(parsed);
	};

	return (
		<>
			<Stack
				component='form'
				noValidate
				onSubmit={submit}
				direction='row'
				sx={{ alignItems: 'flex-start', gap: 1.5, flexWrap: 'wrap' }}
			>
				<TextField
					label={label}
					placeholder={placeholder}
					value={draft}
					slotProps={{ htmlInput: { inputMode: 'numeric' } }}
					onChange={(event) => {
						setDraft(event.target.value);
						setInvalid(false);
					}}
					error={invalid}
					helperText={invalid ? invalidText : undefined}
					sx={{ flex: '0 1 280px' }}
				/>
				<Button
					type='submit'
					variant={connected ? 'outlined' : 'contained'}
					loading={pending}
					disabled={connected && draft.trim() === String(current)}
					sx={{ mt: 3 }}
				>
					{connected ? 'Сохранить' : 'Подключить'}
				</Button>
				{children}
			</Stack>
			{error && <Alert severity='error'>{error}</Alert>}
		</>
	);
}
