import {
	Alert,
	Box,
	Button,
	Card,
	Chip,
	Stack,
	Typography,
} from '@mui/material';
import { useSelector, useForm } from '@tanstack/react-form';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useEffectEvent, useState } from 'react';

import {
	allocationText,
	AllocationWarning,
	AutoExpiredReason,
	deliveryQueryOptions,
	itemName,
	itemUnit,
	reasonLabel,
	storeDelivery,
	WriteOffCommentField,
	WriteOffReasonSelect,
} from '@entities/delivery';
import { ApiError } from '@shared/api';
import { formatDateTime } from '@shared/lib/datetime';
import { isExpired } from '@shared/lib/expiry';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { formatQuantity } from '@shared/lib/quantity';
import { TextLink } from '@shared/ui/link';
import { UnitQuantityField } from '@shared/ui/quantity-field';
import { radius } from '@shared/ui/theme';

import { resolveQuarantine } from '../api/resolve-quarantine';
import {
	closedQuarantineMessage,
	expiredResolution,
	initialResolution,
	quarantineAge,
	resolutionBody,
	resolutionOutcome,
	resolutionProblems,
	type OpenQuarantineItem,
	type QuarantineResolutionBody,
	type Resolution,
	type ResolutionOutcome,
} from '../model/quarantine';

interface QuarantineCardProps {
	pointId: string;
	entry: OpenQuarantineItem;
	now: Date;
	/** Решать может сотрудник с правом приёмки на работающей точке; иначе карточка только для чтения. */
	mayResolve: boolean;
	/** Позиция ушла из карантина: решение сохранено или карантин уже закрыли. */
	onClosed: (message: ClosedMessage) => void;
}

/** Сообщение панели о позиции, которая ушла из карантина. */
export interface ClosedMessage {
	severity: 'success' | 'warning';
	text: string;
}

function actionLabel(
	outcome: ResolutionOutcome,
	resolution: Resolution,
	format: (value: number) => string
): string {
	switch (outcome) {
		case 'accept':
			return `Принять ${format(resolution.accepted ?? 0)} на склад`;

		case 'reject':
			return `Отклонить ${format(resolution.rejected ?? 0)} окончательно`;

		case 'split':
			return 'Принять решение';
	}
}

/** Позиция открытого карантина с решением: принять на склад, отклонить или разделить. */
export function QuarantineCard({
	pointId,
	entry,
	now,
	mayResolve,
	onClosed,
}: QuarantineCardProps) {
	const { delivery, item } = entry;
	const queryClient = useQueryClient();
	const keyFor = useIdempotencyKey();
	const unit = itemUnit(item);
	const name = itemName(item);
	const format = (value: number) => formatQuantity(value, unit);
	const [defaultValues] = useState(() => initialResolution(item, now));
	const form = useForm({ defaultValues });
	const resolution = useSelector(form.store, (state) => state.values);
	const [fieldError, setFieldError] = useState<string>();
	const problems = resolutionProblems(item, resolution);
	const outcome = resolutionOutcome(resolution);
	const expired = isExpired(item.expires_at, now) || resolution.expired;

	const expire = () =>
		form.reset(expiredResolution(item), { keepDefaultValues: true });

	const expireDue = useEffectEvent((at: Date) => {
		if (isExpired(item.expires_at, at) && !form.state.values.expired) {
			expire();
		}
	});

	// Часы панели раз в минуту переводят истёкшую позицию в просроченную.
	useEffect(() => {
		expireDue(now);
	}, [now]);

	const mutation = useMutation({
		mutationFn: ({
			body,
			key,
		}: {
			body: QuarantineResolutionBody;
			key: string;
		}) => resolveQuarantine(pointId, delivery.id, item.id, body, key),
		onSuccess: (updated) => {
			storeDelivery(queryClient, pointId, updated);
			onClosed({ severity: 'success', text: `Решение по «${name}» сохранено` });
		},
		onError: async (error) => {
			if (!(error instanceof ApiError)) {
				return;
			}

			if (error.type === 'quarantine-closed') {
				const fresh = await queryClient
					.fetchQuery({
						...deliveryQueryOptions(pointId, delivery.id),
						staleTime: 0,
					})
					.catch(() => undefined);

				if (fresh) {
					storeDelivery(queryClient, pointId, fresh);
					onClosed({
						severity: 'warning',
						text: closedQuarantineMessage(
							item,
							fresh.items.find((candidate) => candidate.id === item.id)
						),
					});
				}

				return;
			}

			const fieldErrors = Object.values(error.fieldErrors);

			if (fieldErrors.some(({ code }) => code === 'expired')) {
				expire();
				mutation.reset();
				return;
			}

			if (fieldErrors.length > 0) {
				setFieldError(fieldErrors.map(({ message }) => message).join('. '));
				mutation.reset();
			}
		},
	});

	const submit = () => {
		const body = resolutionBody(form.state.values);

		setFieldError(undefined);
		mutation.mutate({ body, key: keyFor(body) });
	};

	const message = allocationText(
		item.quarantined_quantity,
		(resolution.accepted ?? 0) + (resolution.rejected ?? 0),
		unit,
		'quarantine'
	);
	const rejected = (resolution.rejected ?? 0) > 0;

	return (
		<Card
			component='li'
			aria-label={name}
			sx={{ p: 2.5, display: 'grid', gap: 1.75 }}
		>
			<Box>
				<Stack
					direction='row'
					sx={{ gap: 1.25, alignItems: 'center', flexWrap: 'wrap' }}
				>
					<Typography variant='h3' component='h3'>
						{name}
					</Typography>
					{expired && <Chip color='error' label='просрочен' />}
					<Chip
						color={expired ? 'default' : 'warning'}
						label={quarantineAge(entry, now)}
					/>
				</Stack>
				<Typography variant='body2' sx={{ mt: 0.25, color: 'text.secondary' }}>
					<TextLink
						to='/points/$pointId/deliveries/$deliveryId'
						params={{ pointId, deliveryId: delivery.id }}
						underline='hover'
						sx={{ color: 'text.primary', fontWeight: 600 }}
					>
						Поставка № {delivery.number}
					</TextLink>
					{' · '}
					{expired ? 'истёк' : 'годен до'}{' '}
					{formatDateTime(item.expires_at, now)}
					{` · в карантине ${format(item.quarantined_quantity)}`}
				</Typography>
			</Box>
			{item.quarantine_reason && (
				<Box
					sx={(theme) => ({
						px: 1.75,
						py: 1.25,
						borderRadius: radius(theme, 'md'),
						bgcolor: 'field',
						border: '1px solid',
						borderColor: 'divider',
					})}
				>
					<Typography
						variant='caption'
						component='div'
						sx={{ color: 'text.secondary' }}
					>
						Причина карантина
					</Typography>
					<Typography sx={{ color: 'text.strong' }}>
						{reasonLabel(item.quarantine_reason)}
						{item.comment ? ` — «${item.comment}»` : ''}
					</Typography>
				</Box>
			)}
			{mayResolve && (
				<>
					<Box
						sx={{
							display: 'grid',
							gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
							gap: 1.5,
							alignItems: 'start',
						}}
					>
						<form.Field name='accepted'>
							{(field) => (
								<UnitQuantityField
									label='Принять'
									accessibleLabel={`${name}: принять`}
									fullWidth
									unit={unit}
									max={item.quarantined_quantity}
									value={field.state.value}
									onValueChange={(value) => field.handleChange(value)}
									disabled={expired}
									helperText={
										expired ? 'принять нельзя — срок истёк' : undefined
									}
								/>
							)}
						</form.Field>
						<form.Field name='rejected'>
							{(field) => (
								<UnitQuantityField
									label='Отклонить'
									accessibleLabel={`${name}: отклонить`}
									fullWidth
									unit={unit}
									max={item.quarantined_quantity}
									value={field.state.value}
									onValueChange={(value) => field.handleChange(value)}
								/>
							)}
						</form.Field>
					</Box>
					{message && <AllocationWarning>{message}</AllocationWarning>}
					{rejected && (
						<Box
							sx={{
								display: 'grid',
								gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
								gap: 1.5,
							}}
						>
							{resolution.expired ? (
								<AutoExpiredReason />
							) : (
								<form.Field name='rejectReason'>
									{(field) => (
										<WriteOffReasonSelect
											label='Причина отклонения'
											value={field.state.value}
											onChange={(reason) => field.handleChange(reason)}
											onBlur={field.handleBlur}
											missing={problems.includes('reject-reason')}
											error={
												problems.includes('reject-reason') &&
												field.state.meta.isBlurred
											}
										/>
									)}
								</form.Field>
							)}
							<form.Field name='comment'>
								{(field) => (
									<WriteOffCommentField
										value={field.state.value}
										onChange={(value) => field.handleChange(value)}
										onBlur={field.handleBlur}
										missing={problems.includes('comment')}
										error={
											problems.includes('comment') && field.state.meta.isBlurred
										}
									/>
								)}
							</form.Field>
						</Box>
					)}
					{problems.length === 0 && outcome !== 'accept' && (
						<Typography variant='body2'>
							{outcome === 'split'
								? `Принять ${format(resolution.accepted ?? 0)} — новая партия; ${format(resolution.rejected ?? 0)} будут окончательно отклонены`
								: `Все ${format(resolution.rejected ?? 0)} будут окончательно отклонены, на склад ничего не поступит`}
						</Typography>
					)}
					{(fieldError || mutation.isError) && (
						<Alert severity='error'>
							{fieldError ??
								`Не удалось принять решение: ${mutation.error?.message}. Значения остались в карточке.`}
						</Alert>
					)}
					<Button
						variant='contained'
						color={outcome === 'reject' ? 'error' : 'primary'}
						size='large'
						disabled={problems.length > 0}
						loading={mutation.isPending}
						onClick={submit}
					>
						{mutation.isError
							? 'Повторить'
							: actionLabel(outcome, resolution, format)}
					</Button>
				</>
			)}
		</Card>
	);
}
