import { Button, FormHelperText, Stack, TextField } from '@mui/material';
import { useId, useState, type FormEvent, type ReactNode } from 'react';

interface RenameFormProps {
	label: string;
	/** Текущее название: поле начинается с него и возвращается к нему, когда оно меняется. */
	value: string;
	hint?: string;
	pending: boolean;
	/** Текст ошибки сервера. */
	error?: string;
	onSave: (name: string) => void;
	/** Почему менять нельзя (`ReadOnlyNote`, `AccessNote`): тогда вместо поля название показано текстом. */
	readOnlyNote?: ReactNode;
}

/** Поле названия с кнопкой «Сохранить»: пустое не отправляется, без изменений кнопка недоступна. */
export function RenameForm({ readOnlyNote, ...props }: RenameFormProps) {
	if (readOnlyNote) {
		return (
			<>
				<div>{props.value}</div>
				{readOnlyNote}
			</>
		);
	}

	return <NameForm key={props.value} {...props} />;
}

function NameForm({
	label,
	value,
	hint,
	pending,
	error,
	onSave,
}: Omit<RenameFormProps, 'readOnlyNote'>) {
	const helperId = useId();
	const [draft, setDraft] = useState(value);
	const [emptyTried, setEmptyTried] = useState(false);
	const name = draft.trim();
	const fieldError = emptyTried && !name ? 'Введите название' : error;
	const helper = fieldError || hint;

	const submit = (event: FormEvent) => {
		event.preventDefault();

		if (!name) {
			setEmptyTried(true);
			return;
		}

		onSave(name);
	};

	// Подсказка стоит под рядом, чтобы кнопка выравнивалась по полю и в ряд, и после переноса.
	return (
		<Stack component='form' noValidate onSubmit={submit} sx={{ gap: 0.5 }}>
			<Stack
				direction='row'
				sx={{ alignItems: 'flex-end', gap: 1.5, flexWrap: 'wrap' }}
			>
				<TextField
					label={label}
					value={draft}
					onChange={(event) => {
						setDraft(event.target.value);
						setEmptyTried(false);
					}}
					error={!!fieldError}
					slotProps={{
						htmlInput: { 'aria-describedby': helper ? helperId : undefined },
					}}
					sx={{ flex: '1 1 280px', maxWidth: 440 }}
				/>
				<Button
					type='submit'
					variant='outlined'
					loading={pending}
					disabled={name === value}
				>
					Сохранить
				</Button>
			</Stack>
			{helper && (
				<FormHelperText id={helperId} error={!!fieldError}>
					{helper}
				</FormHelperText>
			)}
		</Stack>
	);
}
