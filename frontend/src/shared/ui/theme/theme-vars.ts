import type { Theme } from '@mui/material/styles';

// Тема собрана с cssVariables, поэтому vars есть всегда.
/** CSS-переменные темы для использования в стилях. */
export const themeVars = (theme: Pick<Theme, 'vars'>) => theme.vars!;

/** Кольцо фокуса вокруг поля ввода. */
export const fieldFocusRing = (theme: Pick<Theme, 'vars' | 'alpha'>) =>
	`0 0 0 3px ${theme.alpha(themeVars(theme).palette.primary.main, 0.28)}`;

/** Радиус шкалы для `sx`: число там умножается на `shape.borderRadius`, поэтому нужны пиксели строкой. */
export const radius = (
	theme: Pick<Theme, 'shape'>,
	size: keyof Theme['shape']['radius']
) => `${theme.shape.radius[size]}px`;
