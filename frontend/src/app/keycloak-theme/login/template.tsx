import { Box, Button, Link, Paper, Stack, Typography } from '@mui/material';
import type { Theme } from '@mui/material/styles';
import { useInitialize } from 'keycloakify/login/Template.useInitialize';
import type { TemplateProps } from 'keycloakify/login/TemplateProps';
import { useEffect, type ReactNode } from 'react';

import { StandaloneScreen } from '@shared/ui/standalone-screen';
import {
	coarsePointer,
	fieldFocusRing,
	radius,
	themeVars,
} from '@shared/ui/theme';

import type { I18n } from './i18n';
import type { KcContext } from './KcContext';
import { KcAlert } from './components/kc-alert';
import { KcHtml } from './components/kc-html';

export type GodenTemplateProps = TemplateProps<KcContext, I18n> & {
	subtitleNode?: ReactNode;
	/** Карточка шире обычной, для формы с полями в два столбца. */
	wide?: boolean;
	/** Оформляет обычные поля и кнопки страниц Keycloak без своей вёрстки. */
	nativeControls?: boolean;
};

/** Каркас страниц входа: логотип, карточка с заголовком, сообщение Keycloak и содержимое страницы. */
export function Template(props: GodenTemplateProps) {
	const {
		displayInfo = false,
		displayMessage = true,
		headerNode,
		subtitleNode,
		socialProvidersNode = null,
		infoNode = null,
		documentTitle,
		kcContext,
		i18n,
		doUseDefaultCss,
		children,
		wide = false,
		nativeControls = false,
	} = props;

	const { msg, msgStr } = i18n;
	const { realm, auth, url, message, isAppInitiatedAction } = kcContext;
	const title =
		documentTitle ?? msgStr('loginTitle', realm.displayName || realm.name);

	useEffect(() => {
		document.title = title;
	}, [title]);

	const { isReadyToRender } = useInitialize({ kcContext, doUseDefaultCss });

	if (!isReadyToRender) {
		return null;
	}

	const showAttemptedUsername =
		auth !== undefined && auth.showUsername && !auth.showResetCredentials;
	// Предупреждение о шаге входа не нужно, если пользователь сам открыл этот шаг из приложения.
	const showMessage =
		displayMessage &&
		message !== undefined &&
		(message.type !== 'warning' || !isAppInitiatedAction);

	return (
		<StandaloneScreen>
			<Paper
				component='section'
				sx={{
					width: '100%',
					maxWidth: wide ? 440 : 400,
					boxSizing: 'border-box',
					p: { xs: 3, sm: 4 },
					display: 'flex',
					flexDirection: 'column',
					gap: 2.5,
				}}
			>
				<Stack sx={{ gap: 0.5 }}>
					<Typography variant='h2' component='h1'>
						{headerNode}
					</Typography>
					{subtitleNode && (
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							{subtitleNode}
						</Typography>
					)}
					{showAttemptedUsername && (
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							{auth.attemptedUsername}
							{' · '}
							<Link href={url.loginRestartFlowUrl}>
								{msg('godenRestartLogin')}
							</Link>
						</Typography>
					)}
				</Stack>

				{showMessage && (
					<KcAlert type={message.type}>
						<KcHtml html={message.summary} />
					</KcAlert>
				)}

				{nativeControls ? (
					<Box sx={nativeControlStyles}>{children}</Box>
				) : (
					children
				)}

				{auth !== undefined && auth.showTryAnotherWayLink && (
					<form action={url.loginAction} method='post'>
						<input type='hidden' name='tryAnotherWay' value='on' />
						<Button type='submit' variant='text' fullWidth>
							{msg('godenTryAnotherWay')}
						</Button>
					</form>
				)}

				{socialProvidersNode}

				{displayInfo && (
					<Typography
						variant='body2'
						sx={{ color: 'text.secondary', textAlign: 'center' }}
					>
						{infoNode}
					</Typography>
				)}
			</Paper>
		</StandaloneScreen>
	);
}

/** Каркас для страниц Keycloak без своей вёрстки. */
export function FallbackTemplate(props: GodenTemplateProps) {
	return <Template {...props} nativeControls />;
}

// Обычная разметка страниц Keycloak получает поля и кнопки в цветах и размерах дизайн-системы.
function nativeControlStyles(theme: Theme) {
	const palette = themeVars(theme).palette;
	const textInput =
		'input:not([type="checkbox"]):not([type="radio"]):not([type="hidden"]):not([type="submit"]):not([type="button"])';

	return {
		'& form': { display: 'flex', flexDirection: 'column', gap: 2 },
		'& label': {
			display: 'block',
			mb: 0.75,
			fontSize: theme.typography.body2.fontSize,
			color: palette.text.secondary,
		},
		[`& ${textInput}, & select, & textarea`]: {
			boxSizing: 'border-box',
			width: '100%',
			minHeight: 36,
			px: 1.5,
			py: 0.875,
			font: 'inherit',
			color: palette.text.strong,
			bgcolor: palette.field,
			border: `1px solid ${palette.lineInput}`,
			borderRadius: radius(theme, 'sm'),
			'&:focus': {
				outline: 'none',
				borderColor: palette.primary.ink,
				boxShadow: fieldFocusRing(theme),
			},
			[coarsePointer]: { minHeight: 44 },
		},
		'& input[type="submit"], & button': {
			minHeight: 40,
			px: 2,
			border: 0,
			borderRadius: radius(theme, 'sm'),
			bgcolor: palette.primary.main,
			color: palette.primary.contrastText,
			font: 'inherit',
			fontWeight: 600,
			cursor: 'pointer',
			[coarsePointer]: { minHeight: 48 },
		},
		'& a': { color: palette.primary.ink },
	};
}
