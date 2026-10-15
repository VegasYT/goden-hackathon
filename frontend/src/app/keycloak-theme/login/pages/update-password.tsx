import { Button, Checkbox, FormControlLabel, Stack } from '@mui/material';

import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { KcHtml } from '../components/kc-html';
import { PasswordField } from '../components/password-field';
import { useSubmitOnce } from '../components/use-submit-once';

type UpdatePasswordContext = Extract<
	KcContext,
	{ pageId: 'login-update-password.ftl' }
>;

export function UpdatePassword({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<UpdatePasswordContext>) {
	const { url, messagesPerField, isAppInitiatedAction } = kcContext;
	const { msg } = i18n;
	const { isSubmitting, onSubmit } = useSubmitOnce();

	const fieldError = (name: string) =>
		messagesPerField.existsError(name) ? (
			<KcHtml html={messagesPerField.get(name)} />
		) : undefined;

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			displayMessage={
				!messagesPerField.existsError('password', 'password-confirm')
			}
			headerNode={msg('godenUpdatePasswordTitle')}
		>
			<form
				id='kc-passwd-update-form'
				action={url.loginAction}
				method='post'
				onSubmit={onSubmit}
			>
				<Stack sx={{ gap: 2.5 }}>
					<Stack sx={{ gap: 2 }}>
						<PasswordField
							i18n={i18n}
							id='password-new'
							name='password-new'
							label={msg('passwordNew')}
							autoComplete='new-password'
							autoFocus
							error={messagesPerField.existsError('password')}
							helperText={fieldError('password')}
						/>
						<PasswordField
							i18n={i18n}
							id='password-confirm'
							name='password-confirm'
							label={msg('passwordConfirm')}
							autoComplete='new-password'
							error={messagesPerField.existsError('password-confirm')}
							helperText={fieldError('password-confirm')}
						/>
					</Stack>
					<FormControlLabel
						control={
							<Checkbox name='logout-sessions' value='on' defaultChecked />
						}
						label={msg('logoutOtherSessions')}
					/>
					<Stack sx={{ gap: 1 }}>
						<Button
							type='submit'
							variant='contained'
							size='large'
							fullWidth
							disabled={isSubmitting}
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
