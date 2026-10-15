import { MenuItem, TextField } from '@mui/material';

import {
	reasonLabel,
	writeOffReasons,
	type WriteOffReason,
} from '../model/reason';

interface WriteOffReasonSelectProps {
	label: string;
	value: WriteOffReason | '';
	onChange: (reason: WriteOffReason) => void;
	onBlur?: () => void;
	/** Причина нужна, а не выбрана: подсказка видна сразу. */
	missing: boolean;
	error: boolean;
}

/** Выбор причины отклонения или карантина из списка причин списания. */
export function WriteOffReasonSelect({
	label,
	value,
	onChange,
	onBlur,
	missing,
	error,
}: WriteOffReasonSelectProps) {
	return (
		<TextField
			select
			label={label}
			value={value}
			onChange={(event) => onChange(event.target.value as WriteOffReason)}
			onBlur={onBlur}
			error={error}
			helperText={missing ? 'Выберите причину' : undefined}
			fullWidth
		>
			{writeOffReasons.map((reason) => (
				<MenuItem key={reason} value={reason}>
					{reasonLabel(reason)}
				</MenuItem>
			))}
		</TextField>
	);
}
