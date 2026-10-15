import SvgIcon, { type SvgIconProps } from '@mui/material/SvgIcon';
import type { ElementType } from 'react';

import { icons, type IconName } from './icons';

const sizes = { sm: 16, md: 20, lg: 24 } as const;

export interface IconProps extends Omit<
	SvgIconProps,
	'component' | 'fontSize' | 'children'
> {
	name: IconName;
	/** Заливная версия для активного состояния, если она есть в реестре. */
	filled?: boolean;
	size?: keyof typeof sizes;
	/** Текст для программ чтения с экрана; без него иконка декоративная. */
	label?: string;
}

export function Icon({
	name,
	filled = false,
	size = 'md',
	label,
	sx,
	...rest
}: IconProps) {
	const entry: { outline: ElementType; filled?: ElementType } = icons[name];
	const component = (filled && entry.filled) || entry.outline;
	const a11y = label
		? { role: 'img', 'aria-label': label, 'aria-hidden': false }
		: { 'aria-hidden': true };

	return (
		<SvgIcon
			component={component}
			inheritViewBox
			sx={[{ fontSize: sizes[size] }, ...(Array.isArray(sx) ? sx : [sx])]}
			{...a11y}
			{...rest}
		/>
	);
}
