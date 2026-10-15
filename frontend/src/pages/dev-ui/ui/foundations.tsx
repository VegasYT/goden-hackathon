import { Box, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';

import { Icon, icons, type IconName } from '@shared/ui/icon';
import {
	type ColorSchemeName,
	type ColorTokenName,
	colorTokens,
	contrastPairs,
	pairRatio,
	radius,
	themeVars,
} from '@shared/ui/theme';

import { Section } from './section';

const schemes: ColorSchemeName[] = ['light', 'dark'];

function Swatches({ scheme }: { scheme: ColorSchemeName }) {
	const tokens = colorTokens[scheme];

	return (
		<Box sx={{ flex: 1, minWidth: 280 }}>
			<Typography variant='h3' component='h3' sx={{ mb: 1 }}>
				{scheme === 'light' ? 'Светлая тема' : 'Тёмная тема'}
			</Typography>
			<Box
				sx={(theme) => ({
					display: 'grid',
					gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
					gap: 1,
					p: 1.5,
					borderRadius: radius(theme, 'lg'),
					// Образцы показывают сами значения токенов независимо от текущей темы.
					bgcolor: tokens.canvas,
				})}
			>
				{(Object.keys(tokens) as ColorTokenName[]).map((name) => (
					<Stack
						key={name}
						direction='row'
						sx={{ gap: 1, alignItems: 'center' }}
					>
						<Box
							sx={{
								width: 28,
								height: 28,
								flexShrink: 0,
								borderRadius: 1,
								bgcolor: tokens[name],
								boxShadow: `inset 0 0 0 1px ${tokens.line}`,
							}}
						/>
						<Box sx={{ minWidth: 0 }}>
							<Typography
								variant='caption'
								sx={{ color: tokens.ink1, display: 'block' }}
							>
								{name}
							</Typography>
							<Typography variant='caption' sx={{ color: tokens.ink3 }}>
								{tokens[name]}
							</Typography>
						</Box>
					</Stack>
				))}
			</Box>
		</Box>
	);
}

export function PaletteSection() {
	return (
		<Section
			title='Палитра'
			description='Значения из tokens.ts. В компонентах цвета берутся только из темы.'
		>
			<Stack direction='row' sx={{ gap: 2, flexWrap: 'wrap' }}>
				{schemes.map((scheme) => (
					<Swatches key={scheme} scheme={scheme} />
				))}
			</Stack>
		</Section>
	);
}

export function ContrastSection() {
	return (
		<Section
			title='Контраст'
			description='Пары из автотеста контраста: текст не ниже 4.5:1, графика не ниже 3:1.'
		>
			<Box
				component='table'
				sx={{
					borderCollapse: 'collapse',
					width: '100%',
					'& td, & th': { py: 0.5, pr: 2, textAlign: 'left' },
				}}
			>
				<thead>
					<tr>
						<Typography
							component='th'
							variant='caption'
							sx={{ color: 'text.tertiary' }}
						>
							Пара
						</Typography>
						<Typography
							component='th'
							variant='caption'
							sx={{ color: 'text.tertiary' }}
						>
							Порог
						</Typography>
						{schemes.map((scheme) => (
							<Typography
								key={scheme}
								component='th'
								variant='caption'
								sx={{ color: 'text.tertiary' }}
							>
								{scheme === 'light' ? 'Светлая' : 'Тёмная'}
							</Typography>
						))}
					</tr>
				</thead>
				<tbody>
					{contrastPairs.map((pair) => (
						<tr key={pair.name}>
							<Typography component='td' variant='body2'>
								{pair.name}
							</Typography>
							<Typography component='td' variant='body2'>
								{pair.min}:1
							</Typography>
							{schemes.map((scheme) => {
								const ratio = pairRatio(pair, colorTokens[scheme]);

								return (
									<Typography key={scheme} component='td' variant='body2'>
										{ratio.toFixed(2)}:1
									</Typography>
								);
							})}
						</tr>
					))}
				</tbody>
			</Box>
		</Section>
	);
}

export function TypographySection() {
	return (
		<Section
			title='Типографика'
			description='Golos Text, шаг около 1.25, табличные цифры. На планшете body и кнопки крупнее.'
		>
			<Stack sx={{ gap: 1.5 }}>
				<Typography variant='h1' component='p'>
					h1 · Приёмка поставки № 118
				</Typography>
				<Typography variant='h2' component='p'>
					h2 · Подтвердить приёмку?
				</Typography>
				<Typography variant='h3' component='p'>
					h3 · Позиции
				</Typography>
				<Typography>
					body1 · Поступила 10 октября в 09:40 · 8 позиций · 12,0 кг
				</Typography>
				<Typography variant='body2'>
					body2 · вторичный текст, 360 шт · 1 234,5 л
				</Typography>
				<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
					caption · изготовлен 08.10
				</Typography>
			</Stack>
		</Section>
	);
}

export function RadiusSection() {
	const theme = useTheme();
	const radii = Object.entries(theme.shape.radius);

	return (
		<Section
			title='Радиусы'
			description='xs — чипы, sm — кнопки и поля, md — поля на планшете, lg — карточки, xl — диалоги.'
		>
			<Stack direction='row' sx={{ gap: 2, flexWrap: 'wrap' }}>
				{radii.map(([name, value]) => (
					<Stack key={name} sx={{ alignItems: 'center', gap: 0.5 }}>
						<Box
							sx={{
								width: 64,
								height: 48,
								borderRadius: `${value}px`,
								bgcolor: 'background.paper',
								boxShadow: (t) =>
									`0 0 0 1px ${themeVars(t).palette.lineStrong}`,
							}}
						/>
						<Typography variant='caption'>
							{name} · {value}px
						</Typography>
					</Stack>
				))}
			</Stack>
		</Section>
	);
}

export function IconsSection() {
	return (
		<Section
			title='Иконки'
			description='Реестр shared/ui/icon. Заливная версия есть только у иконок навигации.'
		>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))',
					gap: 1,
				}}
			>
				{(Object.keys(icons) as IconName[]).map((name) => (
					<Stack
						key={name}
						direction='row'
						sx={{ gap: 1, alignItems: 'center', color: 'text.secondary' }}
					>
						<Icon name={name} />
						{'filled' in icons[name] && (
							<Icon name={name} filled sx={{ color: 'primary.ink' }} />
						)}
						<Typography variant='caption' sx={{ color: 'text.primary' }}>
							{name}
						</Typography>
					</Stack>
				))}
			</Box>
		</Section>
	);
}
