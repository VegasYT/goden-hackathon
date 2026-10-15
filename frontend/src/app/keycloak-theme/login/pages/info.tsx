import { Button, Stack, Typography } from '@mui/material';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { KcHtml } from '../components/kc-html';

type InfoContext = Extract<KcContext, { pageId: 'info.ftl' }>;

export function Info({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<InfoContext>) {
	const {
		messageHeader,
		message,
		requiredActions,
		skipLink,
		pageRedirectUri,
		actionUri,
		client,
	} = kcContext;
	const { msg, advancedMsgStr } = i18n;

	const actions = requiredActions
		?.map((action) => advancedMsgStr(`requiredAction.${action}`))
		.join(', ');
	const continueAction = (() => {
		if (skipLink) {
			return undefined;
		}

		if (pageRedirectUri) {
			return { href: pageRedirectUri, label: msg('godenBackToApp') };
		}

		if (actionUri) {
			return { href: actionUri, label: msg('godenDoProceed') };
		}

		if (client.baseUrl) {
			return { href: client.baseUrl, label: msg('godenBackToApp') };
		}

		return undefined;
	})();

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			displayMessage={false}
			headerNode={
				<KcHtml
					html={messageHeader ? advancedMsgStr(messageHeader) : message.summary}
				/>
			}
		>
			<Stack sx={{ gap: 2.5 }}>
				<Typography>
					<KcHtml html={message.summary} />
					{actions && (
						<>
							{' '}
							<b>{actions}</b>
						</>
					)}
				</Typography>
				{continueAction && (
					<Button
						href={continueAction.href}
						variant='contained'
						size='large'
						fullWidth
					>
						{continueAction.label}
					</Button>
				)}
			</Stack>
		</Template>
	);
}
