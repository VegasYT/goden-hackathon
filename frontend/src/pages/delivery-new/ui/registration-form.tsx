import {
	Box,
	Button,
	Card,
	Paper,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	TextField,
	Typography,
} from '@mui/material';
import { useForm, useSelector } from '@tanstack/react-form';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useRef, useState, type ReactNode } from 'react';

import {
	positionsCount,
	registerDelivery,
	storeDelivery,
	type NewDelivery,
} from '@entities/delivery';
import type { Product } from '@entities/product';
import { ApiError } from '@shared/api';
import { useNow } from '@shared/lib/expiry';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { countOf } from '@shared/lib/plural';
import { shallowEqual } from '@shared/lib/shallow-equal';
import { ActionBar } from '@shared/ui/action-bar';
import { useLeaveConfirm } from '@shared/ui/confirm-dialog';
import { DateTimeField } from '@shared/ui/date-field';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PanelHeader } from '@shared/ui/panel-header';
import { RetryAlert } from '@shared/ui/retry-alert';
import { useWideLayout } from '@shared/ui/theme';

import {
	emptyItem,
	formProblems,
	hasProblems,
	headerProblems,
	initialValues,
	isItemExpired,
	itemProblems,
	registrationSummary,
	serverProblems,
	toNewDelivery,
	visibleItemErrors,
	type ItemValues,
	type RegistrationErrors,
} from '../model/registration';
import {
	ExpiresField,
	ManufacturedField,
	ProductField,
	QuantityInput,
	RemoveItemButton,
} from './item-fields';

type RegistrationForm = ReturnType<typeof useRegistrationForm>;

function useRegistrationForm() {
	// Значения по умолчанию считаются один раз: иначе тик часов сбрасывал бы нетронутую форму.
	const [defaultValues] = useState(() => initialValues(new Date()));

	return useForm({ defaultValues });
}

interface ShellProps {
	form: RegistrationForm;
	/** Ошибки показываются после первой попытки отправки. */
	submitted: boolean;
	serverErrors?: RegistrationErrors;
	onEdit: () => void;
}

function DeliveryFields({ form, submitted, serverErrors, onEdit }: ShellProps) {
	const number = useSelector(form.store, (state) => state.values.number);
	const deliveredAt = useSelector(
		form.store,
		(state) => state.values.deliveredAt
	);
	const own = submitted ? headerProblems({ number, deliveredAt }) : undefined;
	const numberError = own?.number ?? serverErrors?.number;
	const deliveredAtError = own?.deliveredAt ?? serverErrors?.deliveredAt;

	return (
		<Paper
			component='section'
			aria-label='Поставка'
			sx={{
				display: 'grid',
				gridTemplateColumns: {
					xs: 'minmax(0, 1fr) minmax(0, 1.25fr)',
					lg: '240px 260px',
				},
				gap: { xs: 2, lg: 3 },
				p: 2,
			}}
		>
			<TextField
				label='Номер поставки'
				value={number}
				onChange={(event) => {
					form.setFieldValue('number', event.target.value);
					onEdit();
				}}
				error={Boolean(numberError)}
				helperText={numberError ?? 'как в накладной'}
			/>
			<DateTimeField
				label='Поступила'
				value={deliveredAt}
				onChange={(next) => {
					form.setFieldValue('deliveredAt', next);
					onEdit();
				}}
				error={Boolean(deliveredAtError)}
				helperText={deliveredAtError}
			/>
		</Paper>
	);
}

interface ItemProps extends ShellProps {
	index: number;
	companyId: string;
	byId: ReadonlyMap<string, Product>;
	onPick: (product: Product) => void;
	now: Date;
	compact: boolean;
	single: boolean;
	onRemove: () => void;
}

/** Поля одной позиции; каждое получает только своё значение и ошибку, чтобы ввод не перерисовывал соседние. */
function useItemFields({
	form,
	index,
	submitted,
	serverErrors,
	companyId,
	byId,
	onPick,
	now,
	compact,
	single,
	onEdit,
	onRemove,
}: ItemProps) {
	const item = useSelector(form.store, (state) => state.values.items[index]);
	const product = item.productId ? byId.get(item.productId) : undefined;
	const errors = visibleItemErrors(
		submitted ? itemProblems(item, product) : undefined,
		serverErrors?.items[index]
	);
	const position = index + 1;
	const update = (patch: Partial<ItemValues>) => {
		form.setFieldValue(`items[${index}]`, (previous) => ({
			...previous,
			...patch,
		}));
		onEdit();
	};
	const common = { position, compact };

	return {
		product: (
			<ProductField
				{...common}
				value={product}
				error={errors.product}
				companyId={companyId}
				onChange={(next) => {
					if (next) {
						onPick(next);
					}

					update({ productId: next?.id ?? null, quantity: null });
				}}
			/>
		),
		quantity: (
			<QuantityInput
				{...common}
				value={item.quantity}
				error={errors.quantity}
				product={product}
				onChange={(next) => update({ quantity: next })}
			/>
		),
		manufactured: (
			<ManufacturedField
				{...common}
				value={item.manufacturedAt}
				error={errors.manufacturedAt}
				onChange={(next) => update({ manufacturedAt: next })}
			/>
		),
		expires: (
			<ExpiresField
				{...common}
				value={item.expiresAt}
				error={errors.expiresAt}
				expired={isItemExpired(item, now)}
				onChange={(next) => update({ expiresAt: next })}
			/>
		),
		remove: (
			<RemoveItemButton
				position={position}
				disabled={single}
				onRemove={onRemove}
			/>
		),
	};
}

function ItemRow(props: ItemProps) {
	const fields = useItemFields(props);

	return (
		<TableRow sx={{ verticalAlign: 'top' }}>
			<TableCell sx={{ pt: 2.25, color: 'text.secondary' }}>
				{props.index + 1}
			</TableCell>
			<TableCell>{fields.product}</TableCell>
			<TableCell>{fields.quantity}</TableCell>
			<TableCell>{fields.manufactured}</TableCell>
			<TableCell>{fields.expires}</TableCell>
			<TableCell align='right'>{fields.remove}</TableCell>
		</TableRow>
	);
}

function ItemCard(props: ItemProps) {
	const fields = useItemFields(props);

	return (
		<Card component='li' sx={{ p: 2, display: 'grid', gap: 1.5 }}>
			<Stack direction='row' sx={{ gap: 1, alignItems: 'flex-start' }}>
				<Box sx={{ flexGrow: 1, minWidth: 0 }}>{fields.product}</Box>
				<Box sx={{ pt: 3.5 }}>{fields.remove}</Box>
			</Stack>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns:
						'minmax(0, 0.9fr) minmax(0, 1fr) minmax(0, 1.3fr)',
					gap: 1.5,
				}}
			>
				{fields.quantity}
				{fields.manufactured}
				{fields.expires}
			</Box>
		</Card>
	);
}

function Summary({
	form,
	submitted,
	serverErrors,
	byId,
	now,
}: ShellProps & { byId: ReadonlyMap<string, Product>; now: Date }) {
	const summary = useSelector(
		form.store,
		(state) =>
			registrationSummary(state.values, byId, now, submitted, serverErrors),
		{ compare: shallowEqual }
	);

	return (
		<Stack sx={{ gap: 0.25 }}>
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				<Box component='span' sx={{ fontWeight: 600, color: 'text.strong' }}>
					{positionsCount(summary.count)}
				</Box>
				{summary.expired > 0 && (
					<Box component='span' sx={{ color: 'warning.ink' }}>
						{' · '}
						{summary.expired} с истёкшим сроком
					</Box>
				)}
			</Typography>
			{(summary.toFix > 0 || summary.headerInvalid) && (
				<Typography role='alert' variant='body2' sx={{ color: 'error.main' }}>
					{summary.toFix > 0
						? `Исправьте ${countOf(summary.toFix, 'позицию', 'позиции', 'позиций')}, чтобы зарегистрировать поставку`
						: 'Исправьте данные поставки, чтобы зарегистрировать её'}
				</Typography>
			)}
		</Stack>
	);
}

interface RegistrationFormProps {
	pointId: string;
	companyId: string;
	header: ReactNode;
}

/** Форма новой поставки по накладной: шапка поставки, позиции, итог и отправка. */
export function RegistrationForm({
	pointId,
	companyId,
	header,
}: RegistrationFormProps) {
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const wide = useWideLayout();
	const now = useNow();
	// Продукты ищутся на сервере, поэтому форма помнит выбранные, чтобы знать их единицы и сроки.
	const [byId, setById] = useState<ReadonlyMap<string, Product>>(new Map());
	const pick = (product: Product) =>
		setById((known) => new Map(known).set(product.id, product));
	const form = useRegistrationForm();
	const keys = useSelector(
		form.store,
		(state) => state.values.items.map((item) => item.key),
		{ compare: shallowEqual }
	);
	const isDirty = useSelector(form.store, (state) => state.isDirty);
	const [submitted, setSubmitted] = useState(false);
	const [serverErrors, setServerErrors] = useState<RegistrationErrors>();
	const keyFor = useIdempotencyKey();
	const registered = useRef(false);

	const mutation = useMutation({
		mutationFn: ({ body, key }: { body: NewDelivery; key: string }) =>
			registerDelivery(pointId, body, key),
		onSuccess: (delivery) => {
			storeDelivery(queryClient, pointId, delivery);
			registered.current = true;
			void navigate({
				to: '/points/$pointId/deliveries/$deliveryId',
				params: { pointId, deliveryId: delivery.id },
				replace: true,
				state: {
					registeredDelivery: {
						number: delivery.number,
						positions: delivery.items.length,
					},
				},
			});
		},
		onError: (error) => {
			const byField =
				error instanceof ApiError
					? serverProblems(error.fieldErrors, form.state.values.items.length)
					: null;

			if (byField) {
				setServerErrors(byField);
				mutation.reset();
			}
		},
	});

	const leaveConfirm = useLeaveConfirm({
		shouldBlock: () => isDirty && !registered.current,
		title: 'Уйти без регистрации?',
		description:
			'Введённые позиции пропадут: поставка ещё не зарегистрирована.',
	});

	const clearServerErrors = () => setServerErrors(undefined);

	const submit = () => {
		const values = form.state.values;

		setSubmitted(true);
		clearServerErrors();

		const body = toNewDelivery(values, byId);

		if (hasProblems(formProblems(values, byId)) || !body) {
			return;
		}

		mutation.mutate({ body, key: keyFor(body) });
	};

	const shell: ShellProps = {
		form,
		submitted,
		serverErrors,
		onEdit: clearServerErrors,
	};
	const itemProps = (index: number): ItemProps => ({
		...shell,
		index,
		companyId,
		byId,
		onPick: pick,
		now,
		compact: wide,
		single: keys.length === 1,
		onRemove: () => {
			form.removeFieldValue('items', index);
			clearServerErrors();
		},
	});

	return (
		<>
			<PageContent
				actionBar={
					<ActionBar
						label='Итог регистрации'
						action={
							<Stack direction='row' sx={{ gap: 1 }}>
								<ButtonLink
									to='/points/$pointId/deliveries'
									params={{ pointId }}
									variant='outlined'
									size='large'
								>
									Отмена
								</ButtonLink>
								<Button
									variant='contained'
									size='large'
									loading={mutation.isPending}
									onClick={submit}
								>
									Зарегистрировать поставку
								</Button>
							</Stack>
						}
					>
						<Summary {...shell} byId={byId} now={now} />
					</ActionBar>
				}
			>
				{header}
				<Stack sx={{ gap: 2 }}>
					<DeliveryFields {...shell} />
					<Paper component='section' aria-labelledby='items-title'>
						<PanelHeader>
							<Typography id='items-title' variant='h3' component='h2'>
								Позиции{' '}
								<Box component='span' sx={{ color: 'text.secondary' }}>
									{keys.length}
								</Box>
							</Typography>
							<Typography variant='caption' sx={{ color: 'text.secondary' }}>
								количество — в кг, л или шт, как у продукта
							</Typography>
						</PanelHeader>
						{wide ? (
							<Table
								aria-label='Позиции поставки'
								sx={{ tableLayout: 'fixed' }}
							>
								<TableHead>
									<TableRow>
										<TableCell sx={{ width: 44 }}>№</TableCell>
										<TableCell>Продукт</TableCell>
										<TableCell sx={{ width: 170 }}>Количество</TableCell>
										<TableCell sx={{ width: 180 }}>
											Изготовлено · необязательно
										</TableCell>
										<TableCell sx={{ width: 230 }}>Годен до</TableCell>
										<TableCell sx={{ width: 60 }} aria-label='Удалить' />
									</TableRow>
								</TableHead>
								<TableBody>
									{keys.map((key, index) => (
										<ItemRow key={key} {...itemProps(index)} />
									))}
								</TableBody>
							</Table>
						) : (
							<Stack
								component='ul'
								aria-label='Позиции поставки'
								sx={{ gap: 1.5, p: 2, m: 0, listStyle: 'none' }}
							>
								{keys.map((key, index) => (
									<ItemCard key={key} {...itemProps(index)} />
								))}
							</Stack>
						)}
						<Box
							sx={{
								px: 2,
								py: 1.5,
								borderTop: '1px solid',
								borderColor: 'divider',
							}}
						>
							<Button
								startIcon={<Icon name='add' />}
								onClick={() => form.pushFieldValue('items', emptyItem())}
							>
								Добавить позицию
							</Button>
						</Box>
					</Paper>
					{mutation.isError && (
						<RetryAlert
							title='Поставку не зарегистрировали'
							error={mutation.error}
							note='Повтор не создаст вторую поставку.'
							onRetry={submit}
						/>
					)}
				</Stack>
			</PageContent>
			{leaveConfirm}
		</>
	);
}
