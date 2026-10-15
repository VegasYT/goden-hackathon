import { Alert, AlertTitle, Box, Button } from '@mui/material';
import type { ReactNode } from 'react';

import { Icon } from '@shared/ui/icon';

interface RetryAlertProps {
	/** Что не получилось: «Заказ не оформлен». */
	title: string;
	error: Error;
	/** Пояснение под ошибкой, например что повтор не создаст второй заказ. */
	note?: ReactNode;
	retryDisabled?: boolean;
	onRetry: () => void;
}

/** Плашка неудачной записи с текстом ошибки и «Повторить». */
export function RetryAlert({
	title,
	error,
	note,
	retryDisabled,
	onRetry,
}: RetryAlertProps) {
	return (
		<Alert
			severity='error'
			action={
				<Button
					color='inherit'
					startIcon={<Icon name='refresh' />}
					disabled={retryDisabled}
					onClick={onRetry}
				>
					Повторить
				</Button>
			}
		>
			<AlertTitle>{title}</AlertTitle>
			{error.message}
			{note && (
				<Box component='span' sx={{ display: 'block', mt: 0.5 }}>
					{note}
				</Box>
			)}
		</Alert>
	);
}
