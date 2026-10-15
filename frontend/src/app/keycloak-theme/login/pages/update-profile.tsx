import { Button, Stack } from '@mui/material';
import { useState } from 'react';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { ProfileFields } from '../components/profile-fields';
import { useSubmitOnce } from '../components/use-submit-once';

type UpdateProfileContext = Extract<
	KcContext,
	{ pageId: 'login-update-profile.ftl' }
>;

export function UpdateProfile({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<UpdateProfileContext>) {
	const { url, messagesPerField, isAppInitiatedAction } = kcContext;
	const { msg } = i18n;
	const { isSubmitting, onSubmit } = useSubmitOnce();
	const [isSubmittable, setIsSubmittable] = useState(false);

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			wide
			displayMessage={messagesPerField.exists('global')}
			headerNode={msg('godenUpdateProfileTitle')}
		>
			<form
				id='kc-update-profile-form'
				action={url.loginAction}
				method='post'
				onSubmit={onSubmit}
			>
				<Stack sx={{ gap: 2.5 }}>
					<ProfileFields
						kcContext={kcContext}
						i18n={i18n}
						onSubmittableChange={setIsSubmittable}
					/>
					<Stack sx={{ gap: 1 }}>
						<Button
							type='submit'
							variant='contained'
							size='large'
							fullWidth
							disabled={!isSubmittable || isSubmitting}
						>
							{msg('godenDoSaveAndContinue')}
						</Button>
						{isAppInitiatedAction && (
							<Button
								type='submit'
								name='cancel-aia'
								value='true'
								variant='outlined'
								size='large'
								fullWidth
								formNoValidate
							>
								{msg('godenDoCancel')}
							</Button>
						)}
					</Stack>
				</Stack>
			</form>
		</Template>
	);
}
