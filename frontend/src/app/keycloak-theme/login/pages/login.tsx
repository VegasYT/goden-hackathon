import {
	Button,
	Checkbox,
	FormControlLabel,
	Link,
	Stack,
	TextField,
} from '@mui/material';

import type { I18n } from '../i18n';
import type { KcContext } from '../KcContext';
import type { GodenPageProps } from '../page-props';
import { KcAlert } from '../components/kc-alert';
import { KcHtml } from '../components/kc-html';
import { PasswordField } from '../components/password-field';
import { useSubmitOnce } from '../components/use-submit-once';

type LoginContext = Extract<KcContext, { pageId: 'login.ftl' }>;

export function Login({
	kcContext,
	i18n,
	Template,
}: GodenPageProps<LoginContext>) {
	const {
		realm,
		url,
		usernameHidden,
		login,
		auth,
		registrationDisabled,
		messagesPerField,
	} = kcContext;
	const { msg } = i18n;
	const { isSubmitting, onSubmit } = useSubmitOnce();
	const usernameFilled = Boolean(login.username);
	const hasCredentialsError = messagesPerField.existsError(
		'username',
		'password'
	);
	const errorId = hasCredentialsError ? 'input-error' : undefined;

	return (
		<Template
			kcContext={kcContext}
			i18n={i18n}
			doUseDefaultCss={false}
			displayMessage={!hasCredentialsError}
			headerNode={msg('godenLoginTitle')}
			subtitleNode={msg('godenTagline')}
			displayInfo={
				realm.password && realm.registrationAllowed && !registrationDisabled
			}
			infoNode={
				<>
					{msg('godenNoAccount')}{' '}
					<Link href={url.registrationUrl} sx={{ fontWeight: 500 }}>
						{msg('godenDoRegister')}
					</Link>
				</>
			}
		>
			{realm.password && (
				<form
					id='kc-form-login'
					action={url.loginAction}
					method='post'
					onSubmit={onSubmit}
				>
					<Stack sx={{ gap: 2.5 }}>
						{hasCredentialsError && (
							<KcAlert type='error' id={errorId}>
								<KcHtml
									html={messagesPerField.getFirstError('username', 'password')}
								/>
							</KcAlert>
						)}
						<Stack sx={{ gap: 2 }}>
							{!usernameHidden && (
								<TextField
									id='username'
									name='username'
									label={usernameLabel(kcContext, i18n)}
									defaultValue={login.username ?? ''}
									autoComplete='username'
									autoFocus={!usernameFilled}
									error={hasCredentialsError}
									fullWidth
									slotProps={{
										inputLabel: { shrink: true },
										htmlInput: { 'aria-describedby': errorId },
									}}
								/>
							)}
							<PasswordField
								i18n={i18n}
								id='password'
								name='password'
								label={msg('password')}
								autoComplete='current-password'
								autoFocus={usernameHidden || usernameFilled}
								error={hasCredentialsError}
								slotProps={{ htmlInput: { 'aria-describedby': errorId } }}
							/>
						</Stack>
						{realm.rememberMe && !usernameHidden && (
							<FormControlLabel
								control={
									<Checkbox
										name='rememberMe'
										defaultChecked={Boolean(login.rememberMe)}
									/>
								}
								label={msg('rememberMe')}
							/>
						)}
						{realm.resetPasswordAllowed && (
							<Link href={url.loginResetCredentialsUrl} variant='body2'>
								{msg('doForgotPassword')}
							</Link>
						)}
						<input
							type='hidden'
							name='credentialId'
							defaultValue={auth.selectedCredential}
						/>
						<Button
							type='submit'
							name='login'
							variant='contained'
							size='large'
							fullWidth
							disabled={isSubmitting}
						>
							{msg('godenDoLogIn')}
						</Button>
					</Stack>
				</form>
			)}
		</Template>
	);
}

function usernameLabel(kcContext: LoginContext, i18n: I18n) {
	const { realm } = kcContext;

	if (!realm.loginWithEmailAllowed) {
		return i18n.msg('username');
	}

	return realm.registrationEmailAsUsername
		? i18n.msg('email')
		: i18n.msg('usernameOrEmail');
}
