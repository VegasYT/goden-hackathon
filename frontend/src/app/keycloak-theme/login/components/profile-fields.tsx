import { Box, TextField } from '@mui/material';
import type { Attribute } from 'keycloakify/login/KcContext';
import { useUserProfileForm } from 'keycloakify/login/lib/useUserProfileForm';
import { useEffect } from 'react';

import type { I18n } from '../i18n';
import type { KcContext } from '../KcContext';
import { PasswordField } from './password-field';

type ProfileContext = Extract<
	KcContext,
	{ pageId: 'register.ftl' | 'login-update-profile.ftl' }
>;

const halfWidthFields = new Set(['firstName', 'lastName']);
const passwordFields = new Set(['password', 'password-confirm']);

/** Поля профиля realm с проверками Keycloak: регистрация и дополнение профиля. */
export function ProfileFields({
	kcContext,
	i18n,
	onSubmittableChange,
}: {
	kcContext: ProfileContext;
	i18n: I18n;
	onSubmittableChange: (submittable: boolean) => void;
}) {
	const {
		formState: { formFieldStates, isFormSubmittable },
		dispatchFormAction,
	} = useUserProfileForm({ kcContext, i18n, doMakeUserConfirmPassword: true });

	useEffect(() => {
		onSubmittableChange(isFormSubmittable);
	}, [isFormSubmittable, onSubmittableChange]);

	return (
		<Box
			sx={{
				display: 'grid',
				gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, minmax(0, 1fr))' },
				columnGap: 1.5,
				rowGap: 2,
			}}
		>
			{formFieldStates.map(
				({ attribute, displayableErrors, valueOrValues }) => {
					const value = Array.isArray(valueOrValues)
						? (valueOrValues[0] ?? '')
						: valueOrValues;

					if (attribute.annotations.inputType === 'hidden') {
						return (
							<input
								key={attribute.name}
								type='hidden'
								name={attribute.name}
								value={value}
							/>
						);
					}

					const error = displayableErrors[0];
					const common = {
						id: attribute.name,
						name: attribute.name,
						label: i18n.advancedMsg(attribute.displayName ?? attribute.name),
						value,
						error: error !== undefined,
						helperText: error?.errorMessage,
						autoComplete: attribute.autocomplete,
						fullWidth: true,
						onChange: (event: { target: { value: string } }) =>
							dispatchFormAction({
								action: 'update',
								name: attribute.name,
								valueOrValues: event.target.value,
							}),
						onBlur: () =>
							dispatchFormAction({
								action: 'focus lost',
								name: attribute.name,
								fieldIndex: undefined,
							}),
						sx: {
							gridColumn: halfWidthFields.has(attribute.name)
								? undefined
								: '1 / -1',
						},
					};

					if (passwordFields.has(attribute.name)) {
						return (
							<PasswordField key={attribute.name} i18n={i18n} {...common} />
						);
					}

					return (
						<TextField
							key={attribute.name}
							{...common}
							type={inputType(attribute)}
							slotProps={{
								inputLabel: { shrink: true },
								htmlInput: { readOnly: attribute.readOnly },
							}}
						/>
					);
				}
			)}
		</Box>
	);
}

// Незнакомый тип поля показывается обычным текстовым полем.
function inputType(attribute: Attribute) {
	const type = attribute.annotations.inputType;

	if (type === 'html5-email' || attribute.name === 'email') {
		return 'email';
	}

	if (type === 'html5-tel') {
		return 'tel';
	}

	if (type === 'html5-number') {
		return 'number';
	}

	return 'text';
}
