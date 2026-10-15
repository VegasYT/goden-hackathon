import { Box, Stack, Typography, type BoxProps } from '@mui/material';
import { createLink, Link } from '@tanstack/react-router';
import { forwardRef, Fragment, type ReactNode } from 'react';

import {
	isPointDisabled,
	roleLabel,
	type Profile,
	type ProfilePoint,
} from '@entities/profile';
import {
	ConnectionBanner,
	ConnectionIndicator,
	type ConnectionState,
} from '@shared/ui/connection-status';
import { Icon, navIconMotion, playNavIconMotion } from '@shared/ui/icon';
import { Logo } from '@shared/ui/logo';
import { ModeToggle, radius, themeVars, useWideLayout } from '@shared/ui/theme';

import { canOpen, navItems, type NavItem } from './nav-items';
import { PointSwitcher } from './point-switcher';
import { Profile as ProfileButton } from './profile';

export interface AppShellProps {
	profile: Profile;
	/** Выбранная точка; без неё в навигации нет разделов. */
	point?: ProfilePoint;
	children: ReactNode;
	/** Состояние связи с сервером; без него индикатор связи не показывается. */
	connection?: ConnectionState;
}

const NavAnchor = createLink(
	forwardRef<HTMLAnchorElement, BoxProps<'a'>>((props, ref) => (
		<Box component='a' ref={ref} {...props} />
	))
);

function NavLink({
	item,
	pointId,
	compact,
}: {
	item: NavItem;
	pointId: string;
	compact: boolean;
}) {
	return (
		<NavAnchor
			to={item.to}
			params={{ pointId }}
			activeOptions={{ exact: item.exact }}
			onMouseEnter={playNavIconMotion}
			sx={(theme) => ({
				display: 'flex',
				alignItems: 'center',
				borderRadius: radius(theme, 'md'),
				color: 'text.secondary',
				textDecoration: 'none',
				'&:hover': { color: 'text.primary', bgcolor: 'track' },
				...navIconMotion,
				'&[aria-current="page"]': {
					color: 'text.strong',
					fontWeight: 500,
					bgcolor: 'background.paper',
					boxShadow: `0 0 0 1px ${themeVars(theme).palette.divider}`,
				},
				...(compact
					? {
							flexDirection: 'column',
							justifyContent: 'center',
							gap: 0.5,
							width: 64,
							minHeight: 58,
							fontSize: theme.typography.caption.fontSize,
						}
					: { gap: 1.25, height: 36, px: 1.25 }),
			})}
		>
			{({ isActive }) => (
				<>
					<Icon
						name={item.icon}
						size={compact ? 'lg' : undefined}
						filled={isActive}
						sx={{ color: isActive ? 'primary.ink' : 'inherit' }}
					/>
					{compact ? (item.shortLabel ?? item.label) : item.label}
				</>
			)}
		</NavAnchor>
	);
}

/** Подпись группы пунктов; в узкой панели вместо неё черта. */
function NavGroupLabel({
	label,
	compact,
}: {
	label: string;
	compact: boolean;
}) {
	if (compact) {
		return <NavDivider compact />;
	}

	return (
		<Typography
			variant='caption'
			component='div'
			noWrap
			sx={{
				px: 1.25,
				pt: 1.75,
				pb: 0.5,
				fontWeight: 500,
				color: 'text.tertiary',
			}}
		>
			{label}
		</Typography>
	);
}

function NavDivider({ compact }: { compact: boolean }) {
	return (
		<Box
			aria-hidden
			sx={{
				alignSelf: 'stretch',
				height: '1px',
				my: 1,
				mx: compact ? 1.5 : 1.25,
				bgcolor: 'divider',
			}}
		/>
	);
}

const logoLinkSx = (wide: boolean) => ({
	display: 'flex',
	px: wide ? 1 : 0,
	pb: 2,
	textDecoration: 'none',
});

function PointDisabledBanner() {
	return (
		<Box
			role='status'
			sx={{
				display: 'flex',
				alignItems: 'center',
				gap: 1.25,
				px: 4,
				py: 1.25,
				bgcolor: 'warning.tint',
				color: 'warning.onTint',
			}}
		>
			<Icon name='pointDisabled' />
			<Typography sx={{ color: 'inherit' }}>
				<Box component='span' sx={{ fontWeight: 600 }}>
					Точка отключена.
				</Box>{' '}
				Данные доступны только для чтения: поставки, заказы и списания на ней не
				оформляются.
			</Typography>
		</Box>
	);
}

/** Оболочка приложения с навигацией слева и прокручиваемым контентом. */
export function AppShell({
	profile,
	point,
	children,
	connection,
}: AppShellProps) {
	const wide = useWideLayout();
	const visibleItems = point
		? navItems.filter((item) => canOpen(point, item))
		: [];
	const role = point && roleLabel(point);

	return (
		<Box
			sx={{
				display: 'grid',
				gridTemplateColumns: wide
					? '232px minmax(0, 1fr)'
					: '76px minmax(0, 1fr)',
				height: '100dvh',
				bgcolor: 'background.default',
			}}
		>
			<Stack
				component='nav'
				aria-label='Основная навигация'
				sx={{
					gap: 0.25,
					py: 2,
					px: wide ? 1.5 : 0.75,
					alignItems: wide ? 'stretch' : 'center',
					borderRight: '1px solid',
					borderColor: 'divider',
					overflowY: 'auto',
				}}
			>
				{point ? (
					<NavAnchor
						to='/points/$pointId'
						params={{ pointId: point.id }}
						aria-label={wide ? undefined : 'Goden — на сводку'}
						sx={logoLinkSx(wide)}
					>
						<Logo compact={!wide} />
					</NavAnchor>
				) : (
					<Box
						component={Link}
						to='/'
						aria-label={wide ? undefined : 'Goden — на сводку'}
						sx={logoLinkSx(wide)}
					>
						<Logo compact={!wide} />
					</Box>
				)}
				<PointSwitcher profile={profile} point={point} compact={!wide} />
				{point &&
					visibleItems.map((item) => (
						<Fragment key={item.to}>
							{item.heading && (
								<NavGroupLabel label={item.heading(point)} compact={!wide} />
							)}
							{item.separated && <NavDivider compact={!wide} />}
							<NavLink item={item} pointId={point.id} compact={!wide} />
						</Fragment>
					))}
				{wide ? (
					<Box sx={{ mt: 'auto' }}>
						{connection && (
							<Box sx={{ px: 1.25, pb: 1.5 }}>
								<ConnectionIndicator state={connection} />
							</Box>
						)}
						<Stack
							direction='row'
							sx={{
								alignItems: 'center',
								gap: 1,
								pt: 1.25,
								px: 0.5,
								borderTop: '1px solid',
								borderColor: 'divider',
							}}
						>
							<ProfileButton compact={false} role={role} />
							<ModeToggle />
						</Stack>
					</Box>
				) : (
					<Stack sx={{ mt: 'auto', alignItems: 'center', gap: 1, pt: 1.5 }}>
						{connection && <ConnectionIndicator state={connection} compact />}
						<ProfileButton compact role={role} />
					</Stack>
				)}
			</Stack>
			<Box
				component='main'
				sx={{
					display: 'flex',
					flexDirection: 'column',
					minWidth: 0,
					overflowY: 'auto',
				}}
			>
				{connection && connection.status !== 'online' && (
					<ConnectionBanner state={connection} />
				)}
				{point && isPointDisabled(profile, point) && <PointDisabledBanner />}
				{children}
			</Box>
		</Box>
	);
}
