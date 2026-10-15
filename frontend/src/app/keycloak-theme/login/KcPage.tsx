import DefaultPage from 'keycloakify/login/DefaultPage';
import { lazy, Suspense } from 'react';

import { useI18n } from './i18n';
import type { KcContext } from './KcContext';
import { ErrorPage } from './pages/error';
import { Info } from './pages/info';
import { Login } from './pages/login';
import { LogoutConfirm } from './pages/logout-confirm';
import { PageExpired } from './pages/page-expired';
import { Register } from './pages/register';
import { UpdatePassword } from './pages/update-password';
import { UpdateProfile } from './pages/update-profile';
import { VerifyEmail } from './pages/verify-email';
import { FallbackTemplate, Template } from './template';

const UserProfileFormFields = lazy(
	() => import('keycloakify/login/UserProfileFormFields')
);

/** Страница входа Keycloak по её `pageId`; страницы без своей вёрстки идут в общий каркас. */
export default function KcPage({ kcContext }: { kcContext: KcContext }) {
	const { i18n } = useI18n({ kcContext });

	// Пока грузится русский перевод, страница не показывается, чтобы не мелькал английский.
	if (i18n.isFetchingTranslations) {
		return null;
	}

	const page = { i18n, Template };

	switch (kcContext.pageId) {
		case 'login.ftl':
			return <Login kcContext={kcContext} {...page} />;

		case 'register.ftl':
			return <Register kcContext={kcContext} {...page} />;

		case 'login-verify-email.ftl':
			return <VerifyEmail kcContext={kcContext} {...page} />;

		case 'login-update-password.ftl':
			return <UpdatePassword kcContext={kcContext} {...page} />;

		case 'login-update-profile.ftl':
			return <UpdateProfile kcContext={kcContext} {...page} />;

		case 'logout-confirm.ftl':
			return <LogoutConfirm kcContext={kcContext} {...page} />;

		case 'login-page-expired.ftl':
			return <PageExpired kcContext={kcContext} {...page} />;

		case 'info.ftl':
			return <Info kcContext={kcContext} {...page} />;

		case 'error.ftl':
			return <ErrorPage kcContext={kcContext} {...page} />;

		default:
			return (
				<Suspense>
					<DefaultPage
						kcContext={kcContext}
						i18n={i18n}
						Template={FallbackTemplate}
						doUseDefaultCss={false}
						UserProfileFormFields={UserProfileFormFields}
						doMakeUserConfirmPassword
					/>
				</Suspense>
			);
	}
}
