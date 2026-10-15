import {
	IconButton,
	ListItemIcon,
	ListItemText,
	Menu,
	MenuItem,
} from '@mui/material';
import { useState } from 'react';

import { Icon, type IconName } from '@shared/ui/icon';

export interface ActionMenuItem {
	icon: IconName;
	label: string;
	/** Пояснение под подписью пункта. */
	hint?: string;
	/** Необратимое действие: пункт в цвете ошибки. */
	danger?: boolean;
	onClick: () => void;
}

/** Кнопка «⋮» строки с меню действий; `label` — подпись кнопки для чтения с экрана. */
export function ActionMenu({
	label,
	items,
}: {
	label: string;
	items: ActionMenuItem[];
}) {
	const [anchor, setAnchor] = useState<HTMLElement | null>(null);

	return (
		<>
			<IconButton
				aria-label={label}
				aria-haspopup='menu'
				aria-expanded={anchor !== null}
				onClick={(event) => setAnchor(event.currentTarget)}
			>
				<Icon name='more' />
			</IconButton>
			<Menu
				anchorEl={anchor}
				open={anchor !== null}
				onClose={() => setAnchor(null)}
			>
				{items.map((item) => (
					<MenuItem
						key={item.label}
						onClick={() => {
							setAnchor(null);
							item.onClick();
						}}
					>
						<ListItemIcon>
							<Icon
								name={item.icon}
								size='sm'
								sx={item.danger ? { color: 'error.ink' } : undefined}
							/>
						</ListItemIcon>
						<ListItemText
							primary={item.label}
							secondary={item.hint}
							slotProps={
								item.danger
									? { primary: { sx: { color: 'error.ink' } } }
									: undefined
							}
						/>
					</MenuItem>
				))}
			</Menu>
		</>
	);
}
