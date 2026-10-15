import { useMediaQuery, type Theme } from '@mui/material';

/** Широкая раскладка — от 1200px: боковая панель навигации и таблицы вместо карточек. */
export function useWideLayout(): boolean {
	return useMediaQuery((theme: Theme) => theme.breakpoints.up('lg'));
}
