import {
	IconButton,
	InputAdornment,
	TextField,
	type TextFieldProps,
} from '@mui/material';
import { useState } from 'react';

import { Icon } from '@shared/ui/icon';

import type { I18n } from '../i18n';

/** Поле пароля с кнопкой показа введённого значения. */
export function PasswordField({
	i18n,
	slotProps,
	...props
}: Omit<TextFieldProps, 'type'> & { i18n: I18n }) {
	const [revealed, setRevealed] = useState(false);

	return (
		<TextField
			{...props}
			type={revealed ? 'text' : 'password'}
			fullWidth
			slotProps={{
				...slotProps,
				inputLabel: { shrink: true },
				input: {
					endAdornment: (
						<InputAdornment position='end'>
							<IconButton
								edge='end'
								size='small'
								aria-label={i18n.msgStr(
									revealed ? 'godenHidePassword' : 'godenShowPassword'
								)}
								aria-pressed={revealed}
								onClick={() => setRevealed((value) => !value)}
							>
								<Icon name={revealed ? 'hidePassword' : 'showPassword'} />
							</IconButton>
						</InputAdornment>
					),
				},
			}}
		/>
	);
}
