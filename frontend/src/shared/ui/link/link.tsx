import {
	Button,
	CardActionArea,
	Link as MuiLink,
	Tab,
	type ButtonProps,
	type CardActionAreaProps,
	type LinkProps as MuiLinkProps,
	type TabProps,
} from '@mui/material';
import { createLink } from '@tanstack/react-router';
import { forwardRef } from 'react';

// Компоненты MUI, которые ведут по адресу роутера с проверкой пути и параметров.

/** Кнопка-ссылка: `<ButtonLink to='/points/$pointId' params={…} variant='outlined'>`. */
export const ButtonLink = createLink(
	forwardRef<HTMLAnchorElement, ButtonProps<'a'>>((props, ref) => (
		<Button component='a' ref={ref} {...props} />
	))
);

/** Текстовая ссылка. */
export const TextLink = createLink(
	forwardRef<HTMLAnchorElement, MuiLinkProps>((props, ref) => (
		<MuiLink ref={ref} {...props} />
	))
);

/** Карточка-ссылка: вся область карточки ведёт по адресу. */
export const CardLink = createLink(
	forwardRef<HTMLAnchorElement, CardActionAreaProps<'a'>>((props, ref) => (
		<CardActionArea component='a' ref={ref} {...props} />
	))
);

/** Вкладка-ссылка внутри MUI `Tabs`: `<TabLink value='expiring' to='…/stock/expiring' params={…} label='Сроки'>`. */
export const TabLink = createLink(
	forwardRef<HTMLAnchorElement, TabProps<'a'>>((props, ref) => (
		<Tab component='a' ref={ref} {...props} />
	))
);
