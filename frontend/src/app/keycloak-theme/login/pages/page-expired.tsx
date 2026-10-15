import { Button, Stack, Typography } from '@mui/material';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';

type PageExpiredContext = Extract<
	KcContext,
	{ pageId: 'login-page-expired.ftl' }
>;

export function PageExpired({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<PageExpiredContext>) {
	const { url } = kcContext;
	const { msg } = i18n;

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			headerNode={msg('godenPageExpiredTitle')}
		>
			<Stack sx={{ gap: 2.5 }}>
				<Typography>{msg('godenPageExpiredText')}</Typography>
				<Stack sx={{ gap: 1 }}>
					<Button
						href={url.loginAction}
						variant='contained'
						size='large'
						fullWidth
					>
						{msg('godenDoContinueLogin')}
					</Button>
					<Button
						href={url.loginRestartFlowUrl}
						variant='outlined'
						size='large'
						fullWidth
					>
						{msg('godenDoRestartLogin')}
					</Button>
				</Stack>
			</Stack>
		</Template>
	);
}
