import { Button, Stack, Typography } from '@mui/material';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { useSubmitOnce } from '../components/use-submit-once';

type LogoutConfirmContext = Extract<
	KcContext,
	{ pageId: 'logout-confirm.ftl' }
>;

export function LogoutConfirm({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<LogoutConfirmContext>) {
	const { url, client, logoutConfirm } = kcContext;
	const { msg } = i18n;
	const { isSubmitting, onSubmit } = useSubmitOnce();

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			headerNode={msg('godenLogoutTitle')}
		>
			<form action={url.logoutConfirmAction} method='post' onSubmit={onSubmit}>
				<Stack sx={{ gap: 2.5 }}>
					<Typography>{msg('godenLogoutText')}</Typography>
					<input type='hidden' name='session_code' value={logoutConfirm.code} />
					<Stack sx={{ gap: 1 }}>
						<Button
							type='submit'
							name='confirmLogout'
							value='confirmLogout'
							variant='contained'
							size='large'
							fullWidth
							disabled={isSubmitting}
						>
							{msg('godenDoLogout')}
						</Button>
						{!logoutConfirm.skipLink && client.baseUrl && (
							<Button
								href={client.baseUrl}
								variant='text'
								size='large'
								fullWidth
							>
								{msg('godenBackToApp')}
							</Button>
						)}
					</Stack>
				</Stack>
			</form>
		</Template>
	);
}
