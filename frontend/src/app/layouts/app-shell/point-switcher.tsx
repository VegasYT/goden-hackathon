import {
	Box,
	ButtonBase,
	Chip,
	Divider,
	ListSubheader,
	Menu,
	MenuItem,
	Typography,
} from '@mui/material';
import { useMatches, useNavigate } from '@tanstack/react-router';
import { Fragment, useId, useState } from 'react';

import {
	isPointDisabled,
	pointsByCompany,
	roleLabel,
	type Profile,
	type ProfilePoint,
} from '@entities/profile';
import { NewCompanyDialog } from '@features/create-company';
import { Icon } from '@shared/ui/icon';
import { useNotice } from '@shared/ui/notice';
import { radius, themeVars } from '@shared/ui/theme';

import { canOpen, navItemOf } from './nav-items';

function pointInitials(name: string): string {
	const words = name
		.split(/\s+/)
		.filter((word) => word.length > 2 || /^\p{Lu}/u.test(word));
	const letters =
		words.length > 1
			? words[0][0] + words[words.length - 1][0]
			: (words[0] ?? name).slice(0, 2);

	return letters.toUpperCase();
}

function DisabledChip() {
	return <Chip label='отключена' sx={{ flexShrink: 0 }} />;
}

function PointLabel({
	point,
	disabled,
}: {
	point?: ProfilePoint;
	disabled: boolean;
}) {
	return (
		<Box sx={{ flex: 1, minWidth: 0, textAlign: 'left' }}>
			<Typography
				variant='caption'
				component='div'
				noWrap
				sx={{ color: 'text.tertiary' }}
			>
				{point ? point.company_name : 'Точка'}
			</Typography>
			<Typography
				variant='body2'
				component='div'
				noWrap
				sx={{ fontWeight: 500, color: 'text.strong' }}
			>
				{point ? point.name : 'Выберите точку'}
			</Typography>
			{disabled && <DisabledChip />}
		</Box>
	);
}

interface PointSwitcherProps {
	profile: Profile;
	/** Выбранная точка; без неё переключатель предлагает выбрать точку. */
	point?: ProfilePoint;
	compact: boolean;
}

/** Выбор точки в навигации: открывает тот же раздел на другой точке, если на нём есть право. */
export function PointSwitcher({ profile, point, compact }: PointSwitcherProps) {
	const [anchor, setAnchor] = useState<HTMLElement | null>(null);
	const notice = useNotice();
	const [creating, setCreating] = useState(false);
	// Владелец компании может завести ещё одну, поэтому меню у него есть даже при одной точке.
	const mayCreateCompany = profile.companies.length > 0;
	const menuId = useId();
	const navigate = useNavigate();
	const lastPath = useMatches({
		select: (matches) => matches[matches.length - 1]?.fullPath ?? '',
	});
	const section = navItemOf(lastPath);
	const disabled = point ? isPointDisabled(profile, point) : false;
	const label = point
		? `Точка: ${point.name}, ${point.company_name}. Сменить точку`
		: 'Выбрать точку';

	const choose = (next: ProfilePoint) => {
		setAnchor(null);

		if (next.id === point?.id) {
			return;
		}

		if (section && canOpen(next, section)) {
			void navigate({ to: section.to, params: { pointId: next.id } });
			return;
		}

		void navigate({ to: '/points/$pointId', params: { pointId: next.id } });

		if (section) {
			notice.show(
				`На точке «${next.name}» нет раздела «${section.label}» — открыта «Сводка»`
			);
		}
	};

	if (point && profile.points.length === 1 && !mayCreateCompany) {
		return compact ? (
			<Box
				role='img'
				aria-label={`Точка: ${point.name}, ${point.company_name}`}
				sx={{ ...initialsSx, mb: 1 }}
			>
				{pointInitials(point.name)}
			</Box>
		) : (
			<Box
				sx={{
					display: 'flex',
					alignItems: 'center',
					gap: 1.25,
					px: 1.25,
					py: 1,
					mb: 1,
				}}
			>
				<Icon name='point' sx={{ color: 'text.secondary' }} />
				<PointLabel point={point} disabled={disabled} />
			</Box>
		);
	}

	const open = anchor !== null;

	return (
		<>
			<ButtonBase
				aria-label={label}
				aria-haspopup='menu'
				aria-controls={open ? menuId : undefined}
				aria-expanded={open}
				onClick={(event) => setAnchor(event.currentTarget)}
				sx={(theme) =>
					compact
						? {
								flexDirection: 'column',
								gap: 0.5,
								width: 64,
								minHeight: 58,
								mb: 1,
								borderRadius: radius(theme, 'md'),
								fontSize: theme.typography.caption.fontSize,
								color: 'text.secondary',
								bgcolor: open ? 'track' : undefined,
							}
						: {
								gap: 1.25,
								px: 1.25,
								py: 1,
								mb: 1,
								borderRadius: radius(theme, 'md'),
								bgcolor: 'background.paper',
								boxShadow: `0 0 0 1px ${themeVars(theme).palette.divider}`,
							}
				}
			>
				{compact ? (
					<>
						<Box component='span' aria-hidden sx={initialsSx}>
							{point ? pointInitials(point.name) : '?'}
						</Box>
						<Box
							component='span'
							aria-hidden
							sx={{ display: 'flex', alignItems: 'center' }}
						>
							Точка
							<Icon name='switchPoint' size='sm' />
						</Box>
					</>
				) : (
					<>
						<Icon name='point' sx={{ color: 'text.secondary' }} />
						<PointLabel point={point} disabled={disabled} />
						<Icon name='switchPoint' sx={{ color: 'text.secondary' }} />
					</>
				)}
			</ButtonBase>
			<Menu
				id={menuId}
				anchorEl={anchor}
				open={open}
				onClose={() => setAnchor(null)}
				anchorOrigin={
					compact
						? { vertical: 'top', horizontal: 'right' }
						: { vertical: 'bottom', horizontal: 'left' }
				}
				slotProps={{ paper: { sx: { width: compact ? 340 : 320 } } }}
			>
				{pointsByCompany(profile).map((group, index) => (
					<Fragment key={group.companyId}>
						{index > 0 && <Divider />}
						<ListSubheader>{group.companyName}</ListSubheader>
						{group.points.map((item) => {
							const selected = item.id === point?.id;

							return (
								<MenuItem
									key={item.id}
									role='menuitemradio'
									aria-checked={selected}
									selected={selected}
									onClick={() => choose(item)}
									sx={{ gap: 1 }}
								>
									<Box sx={{ display: 'flex', width: 20, flexShrink: 0 }}>
										{selected && (
											<Icon name='check' sx={{ color: 'primary.ink' }} />
										)}
									</Box>
									<Box sx={{ flex: 1, minWidth: 0 }}>
										<Typography noWrap>{item.name}</Typography>
										{isPointDisabled(profile, item) && <DisabledChip />}
									</Box>
									<Typography
										variant='caption'
										sx={{ color: 'text.secondary' }}
									>
										{roleLabel(item)}
									</Typography>
								</MenuItem>
							);
						})}
					</Fragment>
				))}
				{mayCreateCompany && <Divider />}
				{mayCreateCompany && (
					<MenuItem
						onClick={() => {
							setAnchor(null);
							setCreating(true);
						}}
						sx={{ gap: 1, fontWeight: 500 }}
					>
						<Box sx={{ display: 'flex', width: 20, flexShrink: 0 }}>
							<Icon name='add' sx={{ color: 'primary.ink' }} />
						</Box>
						Новая компания
					</MenuItem>
				)}
			</Menu>
			{creating && <NewCompanyDialog onClose={() => setCreating(false)} />}
			{notice.element}
		</>
	);
}

const initialsSx = {
	display: 'flex',
	alignItems: 'center',
	justifyContent: 'center',
	width: 32,
	height: 32,
	borderRadius: 2,
	bgcolor: 'primary.tint',
	color: 'primary.onTint',
	fontSize: 13,
	fontWeight: 600,
} as const;
