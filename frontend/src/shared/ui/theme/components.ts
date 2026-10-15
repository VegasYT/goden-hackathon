import type { SvgIconProps } from '@mui/material';
import type { Components, Theme } from '@mui/material/styles';
import { createElement } from 'react';

import { Icon, type IconName } from '@shared/ui/icon';

import { fieldFocusRing, themeVars } from './theme-vars';
import { overlayShadow } from './tokens';
import { coarsePointer } from './typography';

type ThemeArg = { theme: Omit<Theme, 'components'> };

const overlayStyles = ({ theme }: ThemeArg) => ({
	boxShadow: overlayShadow.light,
	backgroundImage: 'none',
	...theme.applyStyles('dark', { boxShadow: overlayShadow.dark }),
});

const icon = (name: IconName) => createElement(Icon, { name });

/** Стрелка выбора из нашего набора: Select передаёт ей класс поворота. */
function SelectIcon(props: SvgIconProps) {
	return createElement(Icon, { ...props, name: 'expandMore' });
}

const headingStyles = ({ theme }: ThemeArg) => ({
	color: themeVars(theme).palette.text.strong,
});

const fieldPadding = {
	padding: '7px 12px',
	[coarsePointer]: { padding: '11px 14px' },
};

/** Фон, скругление, наведение и фокус поля с рамкой; `prefix` — класс рамки компонента. */
function outlinedFieldRoot(theme: Omit<Theme, 'components'>, prefix: string) {
	return {
		backgroundColor: themeVars(theme).palette.field,
		borderRadius: theme.shape.radius.sm,
		[coarsePointer]: { borderRadius: theme.shape.radius.md },
		[`&:hover .${prefix}-notchedOutline`]: {
			borderColor: themeVars(theme).palette.text.secondary,
		},
		'&.Mui-focused': { boxShadow: fieldFocusRing(theme) },
		[`&.Mui-focused .${prefix}-notchedOutline`]: {
			borderColor: themeVars(theme).palette.primary.ink,
			borderWidth: 1,
		},
	};
}

function notchedOutline(theme: Omit<Theme, 'components'>) {
	return {
		top: 0,
		borderColor: themeVars(theme).palette.lineInput,
		'& legend': { display: 'none' },
	};
}

export const components: Components<Omit<Theme, 'components'>> = {
	MuiCssBaseline: {
		styleOverrides: (theme) => ({
			body: {
				fontVariantNumeric: 'tabular-nums',
				WebkitFontSmoothing: 'antialiased',
				MozOsxFontSmoothing: 'grayscale',
			},
			'h1, h2, h3': { textWrap: 'balance' },
			// Рябь отключена, поэтому фокус клавиатуры показываем кольцом.
			':where(a, button, [role="button"], [tabindex]):focus-visible': {
				outline: `2px solid ${themeVars(theme).palette.primary.ink}`,
				outlineOffset: 2,
			},
		}),
	},
	// Заголовки h1–h3 и заголовки диалогов (DialogTitle — это Typography) — цветом заголовков.
	MuiTypography: {
		styleOverrides: {
			h1: headingStyles,
			h2: headingStyles,
			h3: headingStyles,
		},
	},
	MuiButtonBase: {
		defaultProps: { disableRipple: true },
	},
	// Текстовые кнопки красятся цветом темы, а на тёмном уведомлении он не виден.
	MuiSnackbarContent: {
		styleOverrides: {
			action: { '& .MuiButton-root': { color: 'inherit' } },
		},
	},
	MuiButton: {
		defaultProps: { disableElevation: true },
		styleOverrides: {
			root: ({ theme }) => ({
				borderRadius: theme.shape.radius.sm,
				whiteSpace: 'nowrap',
				minHeight: 36,
				paddingInline: 14,
				[coarsePointer]: {
					minHeight: 44,
					paddingInline: 18,
					borderRadius: theme.shape.radius.md,
				},
			}),
			sizeLarge: { minHeight: 40, [coarsePointer]: { minHeight: 48 } },
			sizeSmall: { minHeight: 30, paddingInline: 10 },
			outlined: ({ theme }) => ({
				backgroundColor: themeVars(theme).palette.background.paper,
				borderColor: themeVars(theme).palette.lineStrong,
				color: themeVars(theme).palette.text.strong,
				'&:hover': {
					backgroundColor: themeVars(theme).palette.field,
					borderColor: themeVars(theme).palette.lineStrong,
				},
			}),
			text: ({ theme }) => ({
				color: themeVars(theme).palette.text.primary,
				'&:hover': { backgroundColor: themeVars(theme).palette.field },
			}),
		},
		variants: [
			{
				// Необратимое действие: рамка и текст в цвете ошибки, а не нейтральные.
				props: { variant: 'outlined', color: 'error' },
				style: ({ theme }) => ({
					color: themeVars(theme).palette.error.ink,
					borderColor: themeVars(theme).palette.error.main,
					'&:hover': {
						backgroundColor: themeVars(theme).palette.error.tint,
						borderColor: themeVars(theme).palette.error.main,
					},
				}),
			},
		],
	},
	MuiIconButton: {
		styleOverrides: {
			root: ({ theme }) => ({
				borderRadius: theme.shape.radius.sm,
				color: themeVars(theme).palette.text.secondary,
				[coarsePointer]: { minWidth: 44, minHeight: 44 },
			}),
		},
	},
	MuiTextField: {
		defaultProps: { slotProps: { inputLabel: { shrink: true } } },
	},
	MuiInputLabel: {
		styleOverrides: {
			root: ({ theme }) => ({
				position: 'relative',
				transform: 'none',
				maxWidth: 'none',
				marginBottom: 6,
				fontSize: theme.typography.body2.fontSize,
				color: themeVars(theme).palette.text.secondary,
				'&.Mui-focused': { color: themeVars(theme).palette.text.secondary },
				[coarsePointer]: { fontSize: theme.typography.body1.fontSize },
			}),
			shrink: { transform: 'none' },
		},
	},
	MuiOutlinedInput: {
		styleOverrides: {
			root: ({ theme }) => outlinedFieldRoot(theme, 'MuiOutlinedInput'),
			notchedOutline: ({ theme }) => notchedOutline(theme),
			input: fieldPadding,
		},
	},
	// Поле даты пикеров собрано из секций, а не из <input>, но выглядит как обычное поле.
	MuiPickersOutlinedInput: {
		styleOverrides: {
			root: ({ theme }) => outlinedFieldRoot(theme, 'MuiPickersOutlinedInput'),
			notchedOutline: ({ theme }) => notchedOutline(theme),
			sectionsContainer: {
				padding: '7px 0 7px 12px',
				[coarsePointer]: { padding: '11px 0 11px 14px' },
			},
		},
	},
	MuiChip: {
		defaultProps: { size: 'small' },
		styleOverrides: {
			root: ({ theme }) => ({
				borderRadius: theme.shape.radius.xs,
				fontWeight: 500,
				fontSize: theme.typography.caption.fontSize,
				variants: (['primary', 'success', 'warning', 'error'] as const).map(
					(color) => ({
						props: { color },
						style: {
							backgroundColor: themeVars(theme).palette[color].tint,
							color: themeVars(theme).palette[color].onTint,
						},
					})
				),
			}),
			colorDefault: ({ theme }) => ({
				backgroundColor: 'transparent',
				color: themeVars(theme).palette.text.primary,
				boxShadow: `inset 0 0 0 1px ${themeVars(theme).palette.lineStrong}`,
			}),
		},
	},
	MuiPaper: {
		defaultProps: { elevation: 0 },
		styleOverrides: {
			root: { backgroundImage: 'none' },
			rounded: ({ theme }) => ({ borderRadius: theme.shape.radius.lg }),
			elevation0: ({ theme }) => ({
				boxShadow: `0 0 0 1px ${themeVars(theme).palette.divider}`,
			}),
		},
	},
	MuiTableCell: {
		styleOverrides: {
			root: ({ theme }) => ({
				borderBottom: `1px solid ${themeVars(theme).palette.divider}`,
				padding: '10px 12px',
				[coarsePointer]: { padding: '14px 16px' },
			}),
			head: ({ theme }) => ({
				fontSize: theme.typography.caption.fontSize,
				fontWeight: 500,
				lineHeight: 1.4,
				color: themeVars(theme).palette.text.tertiary,
			}),
			body: ({ theme }) => ({ color: themeVars(theme).palette.text.primary }),
		},
	},
	MuiToggleButtonGroup: {
		styleOverrides: {
			root: ({ theme }) => ({
				backgroundColor: themeVars(theme).palette.track,
				padding: 2,
				gap: 2,
				borderRadius: theme.shape.radius.sm + 1,
				[coarsePointer]: {
					padding: 3,
					borderRadius: theme.shape.radius.md + 2,
				},
			}),
			grouped: ({ theme }) => ({
				border: 0,
				borderRadius: theme.shape.radius.sm - 1,
				[coarsePointer]: {
					borderRadius: theme.shape.radius.md,
				},
			}),
		},
	},
	MuiToggleButton: {
		styleOverrides: {
			root: ({ theme }) => ({
				border: 0,
				padding: '4px 10px',
				fontSize: theme.typography.body2.fontSize,
				fontWeight: 400,
				lineHeight: 1.4,
				color: themeVars(theme).palette.text.secondary,
				'&:hover': {
					backgroundColor: 'transparent',
					color: themeVars(theme).palette.text.primary,
				},
				'&.Mui-selected, &.Mui-selected:hover': {
					backgroundColor: themeVars(theme).palette.background.paper,
					color: themeVars(theme).palette.text.strong,
					fontWeight: 500,
					boxShadow: `0 0 0 1px ${themeVars(theme).palette.divider}`,
				},
				[coarsePointer]: {
					minHeight: 40,
					padding: '8px 14px',
					fontSize: theme.typography.body1.fontSize,
				},
			}),
		},
	},
	MuiLinearProgress: {
		styleOverrides: {
			root: { height: 6, borderRadius: 3 },
			colorPrimary: ({ theme }) => ({
				backgroundColor: themeVars(theme).palette.track,
			}),
			bar: { borderRadius: 3 },
		},
	},
	MuiBackdrop: {
		styleOverrides: {
			root: ({ theme }) => ({
				'&:not(.MuiBackdrop-invisible)': {
					backgroundColor: themeVars(theme).palette.overlay,
				},
			}),
		},
	},
	MuiDialog: {
		styleOverrides: {
			paper: (props) => ({
				...overlayStyles(props),
				borderRadius: props.theme.shape.radius.xl,
			}),
		},
	},
	// Заголовок диалога — h2 дизайн-системы; Dialog сам связывает его с окном через aria-labelledby.
	MuiDialogTitle: {
		defaultProps: { variant: 'h2' },
		styleOverrides: { root: { paddingBottom: 8 } },
	},
	MuiDialogActions: {
		defaultProps: { disableSpacing: true },
		styleOverrides: { root: { padding: '8px 24px 24px', gap: 8 } },
	},
	MuiDrawer: {
		styleOverrides: {
			paper: (props) => ({ ...overlayStyles(props), borderRadius: 0 }),
		},
	},
	MuiPopover: {
		styleOverrides: {
			paper: (props) => ({
				...overlayStyles(props),
				borderRadius: props.theme.shape.radius.md,
			}),
		},
	},
	MuiTooltip: {
		styleOverrides: {
			tooltip: ({ theme }) => ({
				backgroundColor: themeVars(theme).palette.text.strong,
				color: themeVars(theme).palette.background.paper,
				fontSize: theme.typography.caption.fontSize,
				borderRadius: theme.shape.radius.xs,
			}),
		},
	},
	// У выбора из списка свои отступы поля; правый отступ под иконки MUI считает сам.
	MuiAutocomplete: {
		defaultProps: { popupIcon: icon('expandMore'), clearIcon: icon('close') },
		styleOverrides: {
			root: {
				'& .MuiOutlinedInput-root': { paddingBlock: 0, paddingLeft: 0 },
				'& .MuiOutlinedInput-root .MuiAutocomplete-input': fieldPadding,
			},
		},
	},
	MuiSelect: {
		defaultProps: { IconComponent: SelectIcon },
	},
	MuiCheckbox: {
		styleOverrides: {
			root: ({ theme }) => ({
				color: themeVars(theme).palette.lineInput,
				'&.Mui-checked': { color: themeVars(theme).palette.primary.ink },
			}),
		},
	},
	MuiAlert: {
		defaultProps: {
			iconMapping: {
				success: icon('done'),
				warning: icon('warning'),
				error: icon('error'),
				info: icon('info'),
			},
		},
		styleOverrides: {
			root: ({ theme }) => ({
				borderRadius: theme.shape.radius.md,
				variants: [
					...(['warning', 'error', 'success'] as const).map((severity) => ({
						props: { variant: 'standard', severity },
						style: {
							backgroundColor: themeVars(theme).palette[severity].tint,
							color: themeVars(theme).palette[severity].onTint,
							'& .MuiAlert-icon': {
								color: themeVars(theme).palette[severity].onTint,
							},
						},
					})),
					{
						// Пояснение без статуса: спокойная плашка вместо голубого MUI.
						props: { variant: 'standard', severity: 'info' },
						style: {
							backgroundColor: themeVars(theme).palette.field,
							color: themeVars(theme).palette.text.strong,
							boxShadow: `inset 0 0 0 1px ${themeVars(theme).palette.divider}`,
							'& .MuiAlert-icon': {
								color: themeVars(theme).palette.text.secondary,
							},
						},
					},
				],
			}),
		},
	},
};
