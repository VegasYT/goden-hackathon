import { IconButton, InputAdornment, TextField } from '@mui/material';
import { useEffect, useState } from 'react';

import { searchDelayMs, searchTextMaxLength } from '@shared/lib/search-params';
import { Icon } from '@shared/ui/icon';

interface SearchFieldProps {
	value: string;
	onChange: (value: string) => void;
	placeholder: string;
	/** Подпись поля для программ чтения с экрана. */
	label: string;
}

/** Поле поиска списка: отдаёт значение после паузы в наборе, чтобы не менять адрес на каждую букву. */
export function SearchField({
	value,
	onChange,
	placeholder,
	label,
}: SearchFieldProps) {
	const [draft, setDraft] = useState(value);
	const [committed, setCommitted] = useState(value);

	// Значение сменилось снаружи, например «Сбросить поиск».
	if (value !== committed) {
		setCommitted(value);
		setDraft(value);
	}

	useEffect(() => {
		if (draft === value) {
			return;
		}

		const id = window.setTimeout(() => onChange(draft), searchDelayMs);

		return () => window.clearTimeout(id);
	}, [draft, value, onChange]);

	return (
		<TextField
			value={draft}
			onChange={(event) => setDraft(event.target.value)}
			placeholder={placeholder}
			sx={{ width: { xs: '100%', sm: 260 } }}
			slotProps={{
				htmlInput: { 'aria-label': label, maxLength: searchTextMaxLength },
				input: {
					startAdornment: (
						<InputAdornment position='start'>
							<Icon name='search' sx={{ color: 'text.secondary' }} />
						</InputAdornment>
					),
					endAdornment: draft && (
						<InputAdornment position='end'>
							<IconButton
								aria-label='Очистить поиск'
								edge='end'
								onClick={() => {
									setDraft('');
									onChange('');
								}}
							>
								<Icon name='close' size='sm' />
							</IconButton>
						</InputAdornment>
					),
				},
			}}
		/>
	);
}
