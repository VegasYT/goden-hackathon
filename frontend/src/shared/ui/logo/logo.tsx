import { Box } from '@mui/material';
import type { ComponentType, SVGProps } from 'react';

import LogoDark from './logo-dark.svg?react';
import LogoLight from './logo-light.svg?react';
import MarkDark from './mark-dark.svg?react';
import MarkLight from './mark-light.svg?react';

export interface LogoProps {
	/** Только знак без надписи, для узкой панели навигации. */
	compact?: boolean;
}

type Art = ComponentType<SVGProps<SVGSVGElement>>;

// Рисунки из assets/brand: знак 256×290, знак с надписью 688×290 в единицах рисунка.
const variants = {
	mark: { light: MarkLight, dark: MarkDark, aspect: 256 / 290, height: 36 },
	full: { light: LogoLight, dark: LogoDark, aspect: 688 / 290, height: 36 },
};

function ThemedArt({
	light,
	dark,
	width,
	height,
}: {
	light: Art;
	dark: Art;
	width: number;
	height: number;
}) {
	const size = { width, height, flexShrink: 0 };

	return (
		<>
			<Box
				component={light}
				aria-hidden
				sx={(theme) => ({
					...size,
					display: 'block',
					...theme.applyStyles('dark', { display: 'none' }),
				})}
			/>
			<Box
				component={dark}
				aria-hidden
				sx={(theme) => ({
					...size,
					display: 'none',
					...theme.applyStyles('dark', { display: 'block' }),
				})}
			/>
		</>
	);
}

export function Logo({ compact = false }: LogoProps) {
	const { light, dark, aspect, height } = compact
		? variants.mark
		: variants.full;

	return (
		<Box sx={{ display: 'flex', alignItems: 'center' }}>
			<ThemedArt
				light={light}
				dark={dark}
				width={height * aspect}
				height={height}
			/>
			{/* Надпись в логотипе нарисована, название для программ чтения с экрана - текстом. */}
			<Box
				component='span'
				sx={{
					position: 'absolute',
					width: '1px',
					height: '1px',
					overflow: 'hidden',
					clip: 'rect(0 0 0 0)',
				}}
			>
				Goden
			</Box>
		</Box>
	);
}
