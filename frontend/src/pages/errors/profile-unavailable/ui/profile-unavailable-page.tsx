import { Button, Stack } from '@mui/material';

import { ApiError } from '@shared/api';
import { logout } from '@shared/auth';
import { EmptyState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { StandaloneScreen } from '@shared/ui/standalone-screen';

const noAnswerMessage = 'Сервер не ответил. Проверьте подключение и повторите.';

interface ProfileUnavailablePageProps {
	error: unknown;
	onRetry: () => void;
}

export function ProfileUnavailablePage({
	error,
	onRetry,
}: ProfileUnavailablePageProps) {
	return (
		<StandaloneScreen>
			<EmptyState
				icon='cloudOff'
				titleComponent='h1'
				title='Не удалось загрузить профиль'
				description={
					error instanceof ApiError ? error.message : noAnswerMessage
				}
				action={
					<Stack direction='row' sx={{ gap: 1 }}>
						<Button
							variant='contained'
							startIcon={<Icon name='refresh' />}
							onClick={onRetry}
						>
							Повторить
						</Button>
						<Button onClick={() => void logout()}>Выйти</Button>
					</Stack>
				}
			/>
		</StandaloneScreen>
	);
}
