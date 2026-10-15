import { Button, Stack, Typography } from '@mui/material';

import { Icon } from '@shared/ui/icon';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { KcAlert } from '../components/kc-alert';

type VerifyEmailContext = Extract<
	KcContext,
	{ pageId: 'login-verify-email.ftl' }
>;

export function VerifyEmail({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<VerifyEmailContext>) {
	const { url, user, message } = kcContext;
	const { msg } = i18n;
	const email = user?.email;

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			// Напоминание Keycloak о подтверждении почты повторяет текст страницы.
			displayMessage={message?.type !== 'warning'}
			headerNode={msg('godenVerifyEmailTitle')}
			subtitleNode={msg('godenVerifyEmailSubtitle')}
		>
			<Stack sx={{ gap: 2.5 }}>
				<KcAlert type='info' icon={<Icon name='mail' />}>
					{email
						? msg('godenVerifyEmailSent', email)
						: msg('emailVerifyInstruction1')}
				</KcAlert>
				<Stack sx={{ gap: 1 }}>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{msg('godenVerifyEmailNoLetter')}
					</Typography>
					<Button
						href={url.loginAction}
						variant='outlined'
						size='large'
						fullWidth
					>
						{msg('godenDoResendEmail')}
					</Button>
				</Stack>
			</Stack>
		</Template>
	);
}
