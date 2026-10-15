import { TextField } from '@mui/material';

interface WriteOffCommentFieldProps {
	value: string;
	onChange: (value: string) => void;
	onBlur?: () => void;
	/** Комментарий обязателен, а поле пустое: подсказка видна сразу. */
	missing: boolean;
	/** Показать поле и подсказку ошибкой. */
	error: boolean;
}

/** Комментарий к отклонению или карантину позиции поставки. */
export function WriteOffCommentField({
	value,
	onChange,
	onBlur,
	missing,
	error,
}: WriteOffCommentFieldProps) {
	return (
		<TextField
			label='Комментарий'
			value={value}
			onChange={(event) => onChange(event.target.value)}
			onBlur={onBlur}
			error={error}
			helperText={missing ? 'Напишите комментарий — он обязателен' : undefined}
			slotProps={{ htmlInput: { maxLength: 1000 } }}
			fullWidth
		/>
	);
}
