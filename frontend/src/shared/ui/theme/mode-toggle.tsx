import { IconButton, Tooltip } from '@mui/material';

import { Icon } from '@shared/ui/icon';

import { useModeSwitch } from './use-mode-switch';

/** Кнопка переключения светлой и тёмной темы. */
export function ModeToggle() {
	const modeSwitch = useModeSwitch();

	if (!modeSwitch) {
		return null;
	}

	return (
		<Tooltip title={modeSwitch.label}>
			<IconButton aria-label={modeSwitch.label} onClick={modeSwitch.toggle}>
				<Icon name={modeSwitch.icon} />
			</IconButton>
		</Tooltip>
	);
}
