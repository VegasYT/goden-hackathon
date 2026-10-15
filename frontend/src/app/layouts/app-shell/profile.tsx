import {
	Box,
	ButtonBase,
	Divider,
	ListItemIcon,
	Menu,
	MenuItem,
	Typography,
} from '@mui/material';
import { useId, useState } from 'react';

import { logout, useSession } from '@shared/auth';
import { Avatar } from '@shared/ui/avatar';
import { Icon } from '@shared/ui/icon';
import { radius, useModeSwitch } from '@shared/ui/theme';

/** Профиль вошедшего пользователя в навигации; меню с выходом, в узкой панели ещё и с темой. */
export function Profile({
	compact,
	role,
}: {
	compact: boolean;
	/** Подпись пользователя на выбранной точке; без точки её нет. */
	role?: string;
}) {
	const { name } = useSession();
	const modeSwitch = useModeSwitch();
	const [anchor, setAnchor] = useState<HTMLElement | null>(null);
	const menuId = useId();
	const open = anchor !== null;
	const close = () => setAnchor(null);

	return (
		<>
			<ButtonBase
				aria-label={role ? `Профиль: ${name}, ${role}` : `Профиль: ${name}`}
				aria-haspopup='menu'
				aria-controls={open ? menuId : undefined}
				aria-expanded={open}
				onClick={(event) => setAnchor(event.currentTarget)}
				sx={(theme) => ({
					borderRadius: compact ? '50%' : radius(theme, 'md'),
					...(compact
						? { width: 44, height: 44 }
						: {
								flex: 1,
								minWidth: 0,
								justifyContent: 'flex-start',
								gap: 1.25,
								p: 0.5,
								textAlign: 'left',
								'&:hover': { bgcolor: 'track' },
							}),
				})}
			>
				<Avatar name={name} size={compact ? 44 : 32} />
				{!compact && (
					<Box sx={{ minWidth: 0 }}>
						<Typography
							variant='body2'
							noWrap
							sx={{ fontWeight: 500, color: 'text.strong' }}
						>
							{name}
						</Typography>
						{role && (
							<Typography
								variant='caption'
								noWrap
								component='div'
								sx={{ color: 'text.tertiary' }}
							>
								{role}
							</Typography>
						)}
					</Box>
				)}
			</ButtonBase>
			<Menu
				id={menuId}
				anchorEl={anchor}
				open={open}
				onClose={close}
				anchorOrigin={{
					vertical: 'top',
					horizontal: compact ? 'right' : 'left',
				}}
				transformOrigin={{ vertical: 'bottom', horizontal: 'left' }}
			>
				<Box sx={{ px: 2, py: 1, maxWidth: 260 }}>
					<Typography
						variant='body2'
						sx={{ fontWeight: 500, color: 'text.strong' }}
					>
						{name}
					</Typography>
					{role && (
						<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
							{role}
						</Typography>
					)}
				</Box>
				<Divider />
				{compact && modeSwitch && (
					<MenuItem
						onClick={() => {
							modeSwitch.toggle();
							close();
						}}
					>
						<ListItemIcon>
							<Icon name={modeSwitch.icon} />
						</ListItemIcon>
						{modeSwitch.label}
					</MenuItem>
				)}
				<MenuItem onClick={() => void logout()}>
					<ListItemIcon>
						<Icon name='logout' />
					</ListItemIcon>
					Выйти
				</MenuItem>
			</Menu>
		</>
	);
}
