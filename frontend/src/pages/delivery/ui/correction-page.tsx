import {
	Box,
	Button,
	Paper,
	Stack,
	TableCell,
	TextField,
	Typography,
} from '@mui/material';
import { useSelector } from '@tanstack/react-form';
import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useRef, useState } from 'react';

import { actorName } from '@entities/actor';
import {
	deliveryQueryOptions,
	DeliveryStatusChip,
	itemName,
	positionsCount,
	storeDelivery,
	type Delivery,
} from '@entities/delivery';
import { ApiError } from '@shared/api';
import { formatLongDateTime } from '@shared/lib/datetime';
import { useNow } from '@shared/lib/expiry';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { ActionBar } from '@shared/ui/action-bar';
import { ConfirmDialog, useLeaveConfirm } from '@shared/ui/confirm-dialog';
import { ButtonLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { SegmentedControl } from '@shared/ui/segmented-control';

import { correctDelivery } from '../api/correct';
import {
	correctionBody,
	isChanged,
	plannedChange,
	type ApprovalCorrection,
	type ServerHint,
} from '../model/correction';
import {
	useCorrectionForm,
	useCorrectionSummary,
	type CorrectionForm,
} from '../model/correction-form';
import { itemFieldErrors, parts } from '../model/decision';
import { ChangeLines } from './change-lines';
import { DeliveryBreadcrumbs } from './delivery-breadcrumbs';
import { CorrectionCard, CorrectionRow } from './correction-rows';
import { ItemsPanel } from './item-blocks';
import { partLabels } from './parts';

type ItemFilter = 'all' | 'changed' | 'errors';

interface DeliveryCorrectionPageProps {
	pointId: string;
	deliveryId: string;
}

function Header({
	pointId,
	delivery,
}: {
	pointId: string;
	delivery: Delivery;
}) {
	const approval =
		delivery.approved_by && delivery.approved_at
			? `Приёмка: ${actorName(delivery.approved_by)}, ${formatLongDateTime(delivery.approved_at)} · `
			: '';

	return (
		<PageHeader
			breadcrumbs={
				<DeliveryBreadcrumbs
					pointId={pointId}
					delivery={delivery}
					page='Исправление'
				/>
			}
			title={`Исправление приёмки № ${delivery.number}`}
			status={<DeliveryStatusChip status={delivery.status} />}
			meta={`${approval}заявленное количество не меняется — перераспределите его между принятым, отклонённым и карантином`}
		/>
	);
}

/** Изменённые позиции и причина для диалога подтверждения. */
function CorrectionSummary({
	form,
	delivery,
}: {
	form: CorrectionForm;
	delivery: Delivery;
}) {
	const values = useSelector(form.store, (state) => state.values);

	return (
		<Stack sx={{ gap: 2 }}>
			{delivery.items.map((item, index) =>
				isChanged(item, values.items[index]) ? (
					<ChangeLines
						key={item.id}
						item={item}
						change={plannedChange(item, values.items[index])}
						tense='planned'
					/>
				) : null
			)}
			<Box>
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					Причина
				</Typography>
				<Typography>«{values.reason.trim()}»</Typography>
			</Box>
		</Stack>
	);
}

/** Исправление приёмки: перераспределение заявленного количества по позициям с причиной. */
export function DeliveryCorrectionPage({
	pointId,
	deliveryId,
}: DeliveryCorrectionPageProps) {
	const { data: delivery } = useSuspenseQuery(
		deliveryQueryOptions(pointId, deliveryId)
	);
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const now = useNow();
	const form = useCorrectionForm(delivery);
	const [hints, setHints] = useState<Record<string, ServerHint>>({});
	const [itemErrors, setItemErrors] = useState<Record<string, string>>({});
	const { changed, errors: invalid } = useCorrectionSummary(
		form,
		delivery.items,
		now,
		hints
	);
	const hasReason = useSelector(
		form.store,
		(state) => state.values.reason.trim() !== ''
	);
	const isDirty = useSelector(form.store, (state) => state.isDirty);
	const [filter, setFilter] = useState<ItemFilter>('all');
	const [confirmOpen, setConfirmOpen] = useState(false);
	const keyFor = useIdempotencyKey();
	const saved = useRef(false);

	const mutation = useMutation({
		mutationFn: ({ body, key }: { body: ApprovalCorrection; key: string }) =>
			correctDelivery(pointId, deliveryId, body, key),
		onSuccess: (corrected) => {
			saved.current = true;
			storeDelivery(queryClient, pointId, corrected);
			void navigate({
				to: '/points/$pointId/deliveries/$deliveryId',
				params: { pointId, deliveryId },
			});
		},
		onError: (error) => {
			if (!(error instanceof ApiError)) {
				return;
			}

			const byItem = itemFieldErrors(error);

			if (byItem.length === 0) {
				return;
			}

			setHints((previous) => {
				const next = { ...previous };

				byItem.forEach(({ id, code, limit }) => {
					if (code === 'stock-consumed' && limit !== undefined) {
						next[id] = { ...next[id], limit };
					}

					if (code === 'expired') {
						next[id] = { ...next[id], expired: true };
					}
				});

				return next;
			});
			setItemErrors(
				Object.fromEntries(
					byItem
						.filter(
							({ code }) => code !== 'stock-consumed' && code !== 'expired'
						)
						.map(({ id, message }) => [id, message])
				)
			);

			if (byItem.some(({ code }) => code === 'stock-consumed')) {
				void queryClient.refetchQueries(
					deliveryQueryOptions(pointId, deliveryId)
				);
			}

			setConfirmOpen(false);
			mutation.reset();
		},
	});

	const leaveConfirm = useLeaveConfirm({
		shouldBlock: () => isDirty && !saved.current,
		title: 'Уйти без сохранения исправления?',
		description: `Изменения не сохранятся. Приёмка поставки № ${delivery.number} останется прежней.`,
	});

	const indexes = delivery.items.map((_, index) => index);
	const errors = indexes.filter(
		(index) =>
			invalid.includes(index) ||
			itemErrors[delivery.items[index].id] !== undefined
	);
	const visible =
		filter === 'changed' ? changed : filter === 'errors' ? errors : indexes;
	const canSave = changed.length > 0 && errors.length === 0 && hasReason;

	const closeConfirm = () => {
		setConfirmOpen(false);
		mutation.reset();
	};

	const confirm = () => {
		const body = correctionBody(
			delivery.items,
			form.state.values.items,
			form.state.values.reason
		);

		setItemErrors({});
		mutation.mutate({ body, key: keyFor(body) });
	};

	const itemProps = (index: number) => {
		const item = delivery.items[index];

		return {
			form,
			index,
			item,
			now,
			hint: hints[item.id],
			serverError: itemErrors[item.id],
		};
	};

	const caption = () => {
		if (errors.length > 0) {
			return `Исправьте строку «${itemName(delivery.items[errors[0]])}», чтобы сохранить`;
		}

		if (changed.length === 0) {
			return 'Измените распределение хотя бы одной позиции';
		}

		if (!hasReason) {
			return 'Напишите причину исправления';
		}

		return changed.map((index) => itemName(delivery.items[index])).join(', ');
	};

	return (
		<>
			<PageContent
				actionBar={
					<ActionBar
						label='Итог исправления'
						action={
							<Stack direction='row' sx={{ gap: 1 }}>
								<ButtonLink
									to='/points/$pointId/deliveries/$deliveryId'
									params={{ pointId, deliveryId }}
									variant='outlined'
									size='large'
								>
									Отменить
								</ButtonLink>
								<Button
									variant='contained'
									size='large'
									disabled={!canSave}
									onClick={() => setConfirmOpen(true)}
								>
									Сохранить исправление
								</Button>
							</Stack>
						}
					>
						<Box>
							<Typography
								variant='body2'
								sx={{ fontWeight: 600, color: 'text.strong' }}
							>
								Изменено {positionsCount(changed.length)}
								{errors.length > 0 && ` · ${errors.length} с ошибкой`}
							</Typography>
							<Typography variant='caption' sx={{ color: 'text.secondary' }}>
								{caption()}
							</Typography>
						</Box>
					</ActionBar>
				}
			>
				<Header pointId={pointId} delivery={delivery} />
				<Stack sx={{ gap: 2 }}>
					<ItemsPanel
						aside={
							<SegmentedControl
								label='Фильтр позиций'
								value={filter}
								onChange={setFilter}
								options={[
									{ value: 'all', label: 'Все', count: indexes.length },
									{
										value: 'changed',
										label: 'Изменены',
										count: changed.length,
									},
									{ value: 'errors', label: 'С ошибкой', count: errors.length },
								]}
							/>
						}
						columns={
							<>
								<TableCell>Продукт</TableCell>
								<TableCell sx={{ width: 96 }} align='right'>
									Заявлено
								</TableCell>
								<TableCell sx={{ width: 170 }}>Сейчас</TableCell>
								{parts.map((part) => (
									<TableCell key={part} sx={{ width: 124 }}>
										{partLabels[part]}
									</TableCell>
								))}
								<TableCell sx={{ width: 230 }}>Остаток партии</TableCell>
								<TableCell sx={{ width: 56 }} aria-label='Статус' />
							</>
						}
						rows={visible.map((index) => (
							<CorrectionRow
								key={delivery.items[index].id}
								{...itemProps(index)}
							/>
						))}
						cards={visible.map((index) => (
							<CorrectionCard
								key={delivery.items[index].id}
								{...itemProps(index)}
							/>
						))}
					/>
					<Paper
						component='section'
						aria-label='Причина исправления'
						sx={{ p: 2.5 }}
					>
						<form.Field name='reason'>
							{(field) => (
								<TextField
									label='Причина исправления'
									required
									multiline
									minRows={2}
									value={field.state.value}
									onChange={(event) => field.handleChange(event.target.value)}
									helperText='Причина попадёт в историю поставки и журнал аудита'
									slotProps={{ htmlInput: { maxLength: 1000 } }}
									fullWidth
								/>
							)}
						</form.Field>
					</Paper>
				</Stack>
			</PageContent>
			<ConfirmDialog
				open={confirmOpen}
				title={`Сохранить исправление приёмки № ${delivery.number}?`}
				description='Проверьте, что изменится на складе. Исходная приёмка сохранится в истории.'
				confirmLabel={mutation.isError ? 'Повторить' : 'Сохранить исправление'}
				cancelLabel='Вернуться к исправлению'
				onConfirm={confirm}
				onClose={closeConfirm}
				pending={mutation.isPending}
				error={
					mutation.isError
						? `Не удалось сохранить исправление: ${mutation.error.message}. Значения остались на странице.`
						: undefined
				}
			>
				<CorrectionSummary form={form} delivery={delivery} />
			</ConfirmDialog>
			{leaveConfirm}
		</>
	);
}
