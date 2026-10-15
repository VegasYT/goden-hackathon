import { Box } from '@mui/material';

/** Прочерк «—» на месте пустого значения в таблице или карточке. */
export function EmptyValue() {
	return (
		<Box component='span' sx={{ color: 'text.secondary' }}>
			—
		</Box>
	);
}
