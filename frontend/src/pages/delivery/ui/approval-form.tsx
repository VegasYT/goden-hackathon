import {
	Alert,
	Box,
	Button,
	Card,
	LinearProgress,
	Stack,
	TableCell,
	TableRow,
	Typography,
} from '@mui/material';
import { useSelector } from '@tanstack/react-form';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
	useEffect,
	useEffectEvent,
	useRef,
	useState,
	type ReactNode,
} from 'react';

import {
	deliveryQueryOptions,
	itemName,
	itemUnit,
	positionsCount,
	storeDelivery,
	type Delivery,
	type DeliveryItem,
} from '@entities/delivery';
import { ApiError } from '@shared/api';
import { formatDateTime } from '@shared/lib/datetime';
import { isExpired } from '@shared/lib/expiry';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { formatQuantity } from '@shared/lib/quantity';
import { ActionBar } from '@shared/ui/action-bar';
import { ConfirmDialog, useLeaveConfirm } from '@shared/ui/confirm-dialog';
import { ExpiryLabel } from '@shared/ui/expiry-label';
import { Icon } from '@shared/ui/icon';
import { PageContent } from '@shared/ui/page-content';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { useWideLayout } from '@shared/ui/theme';

import { approveDelivery } from '../api/approve';
import {
	approvalBody,
	decisionProblems,
	expireDecision,
	itemFieldErrors,
	parts,
	type DecisionProblem,
	type DeliveryApproval,
} from '../model/decision';
import {
	useApprovalForm,
	useDecision,
	useDecisionIndexes,
	useOutcomeCounts,
	type ApprovalForm,
} from '../model/form';
import { ApprovalSummary } from './approval-summary';
import {
	ItemCardHeader,
	ItemsPanel,
	ItemTitle,
	OutcomeStat,
} from './item-blocks';
import { QuantityInput, ReasonFields } from './item-fields';
import { outcomeParts, partLabels } from './parts';

type ItemFilter = 'all' | 'pending' | 'expired';

interface ApprovalFormProps {
	pointId: string;
	delivery: Delivery;
	now: Date;
	/** Шапка страницы: форма ставит её в общий контейнер страницы над позициями. */
	header: ReactNode;
	/** Поставку уже принял кто-то другой; поставка в кэше перечитывается после вызова. */
	onAlreadyApproved: () => void;
}

interface ItemProps {
	form: ApprovalForm;
	index: number;
	item: DeliveryItem;
	now: Date;
}

function ItemState({ problems }: { problems: DecisionProblem[] }) {
	return problems.length === 0 ? (
		<Icon name='check' label='Позиция решена' sx={{ color: 'primary.ink' }} />
	) : (
		<Icon name='warning' label='Нужно решение' sx={{ color: 'warning.ink' }} />
	);
}

// Строка и карточка читают только своё решение, поэтому ввод в одной позиции не перерисовывает остальные.
function ItemRow({ form, index, item, now }: ItemProps) {
	const decision = useDecision(form, index);
	const problems = decisionProblems(item, decision);
	const fieldProps = { form, index, item, decision };

	return (
		<TableRow sx={{ verticalAlign: 'top' }}>
			<TableCell>
				<ItemTitle item={item} />
			</TableCell>
			<TableCell align='right'>
				{formatQuantity(item.declared_quantity, itemUnit(item))}
			</TableCell>
			<TableCell>
				<ExpiryLabel expiresAt={item.expires_at} now={now} />
			</TableCell>
			{parts.map((part) => (
				<TableCell key={part}>
					<QuantityInput {...fieldProps} part={part} hideLabel />
				</TableCell>
			))}
			<TableCell>
				<ReasonFields {...fieldProps} problems={problems} />
			</TableCell>
			<TableCell>
				<ItemState problems={problems} />
			</TableCell>
		</TableRow>
	);
}

function ItemCard({ form, index, item, now }: ItemProps) {
	const decision = useDecision(form, index);
	const problems = decisionProblems(item, decision);
	const fieldProps = { form, index, item, decision };

	return (
		<Card component='li' sx={{ p: 2.5, display: 'grid', gap: 1.5 }}>
			<ItemCardHeader
				item={item}
				expiry={<ExpiryLabel expiresAt={item.expires_at} now={now} />}
				status={<ItemState problems={problems} />}
			/>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
					gap: 1.5,
				}}
			>
				{parts.map((part) => (
					<QuantityInput key={part} {...fieldProps} part={part} />
				))}
			</Box>
			<ReasonFields {...fieldProps} problems={problems} />
		</Card>
	);
}

/** Форма приёмки ожидающей поставки: решения по позициям, итог и подтверждение. */
export function ApproveDeliveryForm({
	pointId,
	delivery,
	now,
	header,
	onAlreadyApproved,
}: ApprovalFormProps) {
	const queryClient = useQueryClient();
	const wide = useWideLayout();
	const form = useApprovalForm(delivery, now);
	const pending = useDecisionIndexes(form, delivery.items, 'pending');
	const expired = useDecisionIndexes(form, delivery.items, 'expired');
	const expiredDuringFill = useDecisionIndexes(
		form,
		delivery.items,
		'expiredDuringFill'
	);
	const outcomeCounts = useOutcomeCounts(form, delivery.items);
	const isDirty = useSelector(form.store, (state) => state.isDirty);
	const [filter, setFilter] = useState<ItemFilter>('all');
	const [confirmOpen, setConfirmOpen] = useState(false);
	const [itemErrors, setItemErrors] = useState<Record<string, string>>({});
	const keyFor = useIdempotencyKey();
	const alreadyApproved = useRef(false);

	const expireItems = (itemIds: string[]) => {
		delivery.items.forEach((item, index) => {
			const decision = form.getFieldValue(`items[${index}]`);

			if (itemIds.includes(item.id) && !decision.expired) {
				form.setFieldValue(`items[${index}]`, expireDecision(decision), {
					dontUpdateMeta: true,
				});
			}
		});
	};

	const expireDue = useEffectEvent((at: Date) => {
		expireItems(
			delivery.items
				.filter((item) => isExpired(item.expires_at, at))
				.map((item) => item.id)
		);
	});

	// Часы страницы раз в минуту переводят истёкшие позиции в просроченные.
	useEffect(() => {
		expireDue(now);
	}, [now]);

	const mutation = useMutation({
		mutationFn: ({ body, key }: { body: DeliveryApproval; key: string }) =>
			approveDelivery(pointId, delivery.id, body, key),
		onSuccess: (approved) => storeDelivery(queryClient, pointId, approved),
		onError: (error) => {
			if (!(error instanceof ApiError)) {
				return;
			}

			if (error.type === 'delivery-already-approved') {
				alreadyApproved.current = true;
				setConfirmOpen(false);
				onAlreadyApproved();
				void queryClient.refetchQueries(
					deliveryQueryOptions(pointId, delivery.id)
				);
				return;
			}

			const byItem = itemFieldErrors(error);

			if (byItem.length === 0) {
				return;
			}

			const expired = byItem.filter(({ code }) => code === 'expired');

			expireItems(expired.map(({ id }) => id));
			setItemErrors(
				Object.fromEntries(
					byItem
						.filter(({ code }) => code !== 'expired')
						.map(({ id, message }) => [id, message])
				)
			);
			setConfirmOpen(false);
			mutation.reset();
		},
	});

	const leaveConfirm = useLeaveConfirm({
		shouldBlock: () => isDirty && !alreadyApproved.current,
		title: 'Уйти без подтверждения приёмки?',
		description: `Решения по позициям не сохранятся. Поставка № ${delivery.number} останется ждать приёмки.`,
	});

	const indexes = delivery.items.map((_, index) => index);
	const visible =
		filter === 'pending' ? pending : filter === 'expired' ? expired : indexes;
	const decided = indexes.length - pending.length;

	const closeConfirm = () => {
		setConfirmOpen(false);
		mutation.reset();
	};

	const confirm = () => {
		const body = approvalBody(delivery.items, form.state.values.items);

		setItemErrors({});
		mutation.mutate({ body, key: keyFor(body) });
	};

	return (
		<>
			<PageContent
				actionBar={
					<ActionBar
						label='Итог приёмки'
						action={
							<Button
								variant='contained'
								size='large'
								disabled={pending.length > 0}
								onClick={() => setConfirmOpen(true)}
							>
								Подтвердить приёмку
							</Button>
						}
					>
						<Stack
							direction='row'
							sx={{ gap: 3, alignItems: 'center', flexWrap: 'wrap' }}
						>
							<Box sx={{ minWidth: 180 }}>
								<Typography
									variant='body2'
									sx={{ fontWeight: 600, color: 'text.strong' }}
								>
									{decided} из {positionsCount(indexes.length)} решено
								</Typography>
								<LinearProgress
									variant='determinate'
									value={(decided / Math.max(indexes.length, 1)) * 100}
									aria-hidden
									sx={{ mt: 0.75 }}
								/>
								{!wide && (
									<Typography
										variant='caption'
										component='div'
										sx={{ mt: 0.75, color: 'text.secondary' }}
									>
										{outcomeParts
											.map(
												({ part, label }) =>
													`${outcomeCounts[part]} ${label.toLowerCase()}`
											)
											.join(' · ')}
									</Typography>
								)}
							</Box>
							{wide &&
								outcomeParts.map(({ part, label, icon }) => (
									<OutcomeStat
										key={part}
										icon={icon}
										label={label}
										value={positionsCount(outcomeCounts[part])}
									/>
								))}
						</Stack>
					</ActionBar>
				}
			>
				{header}
				<Stack sx={{ gap: 2 }}>
					{expiredDuringFill.map((index) => {
						const item = delivery.items[index];

						return (
							<Alert key={item.id} severity='warning'>
								Срок годности «{itemName(item)}» истёк{' '}
								{formatDateTime(item.expires_at)}, пока заполнялась приёмка.
								Принять позицию нельзя — распределите её между отклонением и
								карантином.
							</Alert>
						);
					})}
					{Object.entries(itemErrors).map(([id, message]) => {
						const item = delivery.items.find(
							(candidate) => candidate.id === id
						);

						return (
							<Alert key={id} severity='error'>
								{item ? `${itemName(item)}: ` : ''}
								{message}
							</Alert>
						);
					})}
					<ItemsPanel
						aside={
							<SegmentedControl
								label='Фильтр позиций'
								value={filter}
								onChange={setFilter}
								options={[
									{ value: 'all', label: 'Все', count: indexes.length },
									{
										value: 'pending',
										label: 'Требуют решения',
										count: pending.length,
									},
									{
										value: 'expired',
										label: 'Просрочены',
										count: expired.length,
									},
								]}
							/>
						}
						columns={
							<>
								<TableCell>Продукт</TableCell>
								<TableCell sx={{ width: 96 }} align='right'>
									Заявлено
								</TableCell>
								<TableCell sx={{ width: 140 }}>Годен до</TableCell>
								{parts.map((part) => (
									<TableCell key={part} sx={{ width: 124 }}>
										{partLabels[part]}
									</TableCell>
								))}
								<TableCell sx={{ width: 260 }}>Причина и комментарий</TableCell>
								<TableCell sx={{ width: 56 }} aria-label='Статус' />
							</>
						}
						rows={visible.map((index) => (
							<ItemRow
								key={delivery.items[index].id}
								form={form}
								index={index}
								item={delivery.items[index]}
								now={now}
							/>
						))}
						cards={visible.map((index) => (
							<ItemCard
								key={delivery.items[index].id}
								form={form}
								index={index}
								item={delivery.items[index]}
								now={now}
							/>
						))}
					/>
				</Stack>
			</PageContent>
			<ConfirmDialog
				open={confirmOpen}
				title={`Подтвердить приёмку поставки № ${delivery.number}?`}
				description='Проверьте итог. На склад попадёт только принятое количество.'
				confirmLabel={mutation.isError ? 'Повторить' : 'Подтвердить приёмку'}
				cancelLabel='Вернуться к приёмке'
				onConfirm={confirm}
				onClose={closeConfirm}
				pending={mutation.isPending}
				error={
					mutation.isError
						? `Не удалось подтвердить приёмку: ${mutation.error.message}. Решения остались на экране.`
						: undefined
				}
				dismissibleNotice={{
					id: 'delivery-approval',
					text: 'После подтверждения приёмку нельзя изменить — только исправить отдельным действием с указанием причины.',
				}}
			>
				<ApprovalSummary form={form} items={delivery.items} />
			</ConfirmDialog>
			{leaveConfirm}
		</>
	);
}
