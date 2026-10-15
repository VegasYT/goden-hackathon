import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { DatePicker } from '@mui/x-date-pickers/DatePicker';
import { DateTimePicker } from '@mui/x-date-pickers/DateTimePicker';
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider';
import { ruRU } from '@mui/x-date-pickers/locales';
import { ru } from 'date-fns/locale/ru';
import type { ReactNode } from 'react';

interface DateFieldProps {
	label: string;
	value: Date | null;
	onChange: (value: Date | null) => void;
	/** Видимую подпись даёт окружение, например заголовок колонки таблицы. */
	hideLabel?: boolean;
	fullWidth?: boolean;
	error?: boolean;
	helperText?: ReactNode;
	/** С чего начинается календарь и какое время подставляется при выборе дня. */
	referenceDate?: Date;
}

const hiddenLabelSx = {
	'& .MuiInputLabel-root': {
		position: 'absolute',
		width: 1,
		height: 1,
		overflow: 'hidden',
		clip: 'rect(0 0 0 0)',
	},
};

const russianTexts =
	ruRU.components.MuiLocalizationProvider.defaultProps.localeText;

// Провайдер дат живёт в полях, а не в корне: так date-fns грузится только со страницами, где есть даты.
function Russian({ children }: { children: ReactNode }) {
	return (
		<LocalizationProvider
			dateAdapter={AdapterDateFns}
			adapterLocale={ru}
			localeText={russianTexts}
		>
			{children}
		</LocalizationProvider>
	);
}

function textFieldProps({
	hideLabel,
	fullWidth,
	error,
	helperText,
}: DateFieldProps) {
	return {
		fullWidth,
		error,
		helperText,
		sx: hideLabel ? hiddenLabelSx : undefined,
	};
}

/** Дата «дд.мм.гггг» с календарём; неполный ввод значение не меняет. */
export function DateField(props: DateFieldProps) {
	return (
		<Russian>
			<DatePicker
				label={props.label}
				value={props.value}
				onChange={props.onChange}
				referenceDate={props.referenceDate}
				format='dd.MM.yyyy'
				slotProps={{ textField: textFieldProps(props) }}
			/>
		</Russian>
	);
}

/** Дата и время «дд.мм.гггг чч:мм» с календарём; неполный ввод значение не меняет. */
export function DateTimeField(props: DateFieldProps) {
	return (
		<Russian>
			<DateTimePicker
				label={props.label}
				value={props.value}
				onChange={props.onChange}
				referenceDate={props.referenceDate}
				format='dd.MM.yyyy HH:mm'
				ampm={false}
				slotProps={{ textField: textFieldProps(props) }}
			/>
		</Russian>
	);
}
