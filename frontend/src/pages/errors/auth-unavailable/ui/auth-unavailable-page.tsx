import { Button } from '@mui/material';

import { EmptyState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { StandaloneScreen } from '@shared/ui/standalone-screen';

export function AuthUnavailablePage() {
	return (
		<StandaloneScreen>
			<EmptyState
				icon='cloudOff'
				titleComponent='h1'
				title='Не удалось связаться с сервером входа'
				description='Проверьте подключение и повторите. Если не помогло, сообщите администратору.'
				action={
					<Button
						variant='contained'
						startIcon={<Icon name='refresh' />}
						onClick={() => window.location.reload()}
					>
						Повторить
					</Button>
				}
			/>
		</StandaloneScreen>
	);
}
