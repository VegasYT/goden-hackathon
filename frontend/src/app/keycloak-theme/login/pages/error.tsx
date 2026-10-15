import { Button, Stack } from '@mui/material';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { KcAlert } from '../components/kc-alert';
import { KcHtml } from '../components/kc-html';

type ErrorContext = Extract<KcContext, { pageId: 'error.ftl' }>;

export function ErrorPage({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<ErrorContext>) {
	const { message, client, skipLink } = kcContext;
	const { msg } = i18n;
	const backUrl = !skipLink ? client?.baseUrl : undefined;

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			displayMessage={false}
			headerNode={msg('godenErrorTitle')}
		>
			<Stack sx={{ gap: 2.5 }}>
				<KcAlert type='error'>
					<KcHtml html={message.summary} />
				</KcAlert>
				{backUrl && (
					<Button href={backUrl} variant='outlined' size='large' fullWidth>
						{msg('godenBackToApp')}
					</Button>
				)}
			</Stack>
		</Template>
	);
}
