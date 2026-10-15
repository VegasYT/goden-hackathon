import { Button, Link, Stack } from '@mui/material';
import { useState } from 'react';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { ProfileFields } from '../components/profile-fields';
import { useSubmitOnce } from '../components/use-submit-once';

type RegisterContext = Extract<KcContext, { pageId: 'register.ftl' }>;

export function Register({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<RegisterContext>) {
	const { url, messagesPerField, messageHeader } = kcContext;
	const { msg, advancedMsg } = i18n;
	const { isSubmitting, onSubmit } = useSubmitOnce();
	const [isSubmittable, setIsSubmittable] = useState(false);

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			wide
			displayMessage={messagesPerField.exists('global')}
			headerNode={
				messageHeader !== undefined
					? advancedMsg(messageHeader)
					: msg('godenRegisterTitle')
			}
			subtitleNode={msg('godenRegisterSubtitle')}
			displayInfo
			infoNode={
				<>
					{msg('godenHaveAccount')}{' '}
					<Link href={url.loginUrl} sx={{ fontWeight: 500 }}>
						{msg('godenDoLogIn')}
					</Link>
				</>
			}
		>
			<form
				id='kc-register-form'
				action={url.registrationAction}
				method='post'
				onSubmit={onSubmit}
			>
				<Stack sx={{ gap: 2.5 }}>
					<ProfileFields
						kcContext={kcContext}
						i18n={i18n}
						onSubmittableChange={setIsSubmittable}
					/>
					<Button
						type='submit'
						variant='contained'
						size='large'
						fullWidth
						disabled={!isSubmittable || isSubmitting}
					>
						{msg('godenDoRegister')}
					</Button>
				</Stack>
			</form>
		</Template>
	);
}
