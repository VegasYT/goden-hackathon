import {
	Alert,
	Box,
	Button,
	Chip,
	Paper,
	Stack,
	Typography,
} from '@mui/material';
import { useForm } from '@tanstack/react-form';
import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from '@tanstack/react-query';
import { useState } from 'react';

import { ApiError } from '@shared/api';
import { formatDateTime } from '@shared/lib/datetime';
import { countOf } from '@shared/lib/plural';
import { Icon, type IconName } from '@shared/ui/icon';
import { PanelHeader } from '@shared/ui/panel-header';
import { QuantityField } from '@shared/ui/quantity-field';

import {
	generatorKey,
	generatorQueryOptions,
	saveGenerator,
	type GeneratorSettings,
	type GeneratorUpdate,
} from '../api/generator';
import { maxRate, rateError } from '../model/rate';

interface RateValues {
	deliveries: number | null;
	orders: number | null;
}

type RateField = keyof RateValues;

const serverFields: Record<string, RateField> = {
	deliveries_per_minute: 'deliveries',
	orders_per_minute: 'orders',
};

const rateLabels: Record<RateField, string> = {
	deliveries: 'Поставок в минуту',
	orders: 'Заказов в минуту',
};

const rateHint = `от 0 до ${maxRate}`;

const hints: { icon: IconName; text: string }[] = [
	{
		icon: 'info',
		text: 'Свои поставки генератор принимает сам, поэтому в «Ожидают приёмки» они не попадают. Для ручной приёмки привезите тестовую поставку.',
	},
	{
		icon: 'empty',
		text: 'Пустой каталог компании генератор при запуске заполняет демонстрационным набором продуктов и блюд.',
	},
	{
		icon: 'orders',
		text: 'Часть заказов намеренно больше остатков: так проверяются отказ и стоп-лист.',
	},
];

function ratesOf(settings: GeneratorSettings): RateValues {
	return {
		deliveries: settings.deliveries_per_minute,
		orders: settings.orders_per_minute,
	};
}

function StatusChip({
	running,
	disabled,
}: {
	running: boolean;
	disabled: boolean;
}) {
	if (running && disabled) {
		return <Chip color='warning' label='включён, не работает' />;
	}

	return running ? (
		<Chip color='success' label='работает' />
	) : (
		<Chip label='остановлен' />
	);
}

function summary(settings: GeneratorSettings, disabled: boolean): string {
	if (!settings.running) {
		return 'Поставки и заказы на точке сейчас не создаются.';
	}

	const deliveries = countOf(
		settings.deliveries_per_minute,
		'поставку',
		'поставки',
		'поставок'
	);
	const orders = countOf(
		settings.orders_per_minute,
		'заказ',
		'заказа',
		'заказов'
	);

	if (disabled) {
		return `Настроен на ${deliveries} и ${orders} в минуту.`;
	}

	const actions = [
		settings.deliveries_per_minute > 0 && `привозит ${deliveries}`,
		settings.orders_per_minute > 0 && `оформляет ${orders}`,
	].filter(Boolean);

	return actions.length > 0
		? `Каждую минуту ${actions.join(' и ')}.`
		: 'Работает, но при нулевой интенсивности ничего не создаёт.';
}

interface GeneratorCardProps {
	pointId: string;
	/** Точка или её компания отключены: генератор можно только остановить. */
	disabled: boolean;
}

/** Запуск, остановка и интенсивность генератора событий на точке. */
export function GeneratorCard({ pointId, disabled }: GeneratorCardProps) {
	const queryClient = useQueryClient();
	const { data: settings } = useSuspenseQuery(generatorQueryOptions(pointId));
	const [serverErrors, setServerErrors] = useState<
		Partial<Record<RateField, string>>
	>({});

	const save = useMutation({
		mutationFn: (body: GeneratorUpdate) => saveGenerator(pointId, body),
		onMutate: () => setServerErrors({}),
		onSuccess: (saved) => {
			queryClient.setQueryData(generatorKey(pointId), saved);
			form.reset(ratesOf(saved));
		},
		onError: (error) => {
			if (error instanceof ApiError) {
				setServerErrors(
					Object.fromEntries(
						Object.entries(error.fieldErrors).flatMap(([name, { message }]) =>
							serverFields[name] ? [[serverFields[name], message]] : []
						)
					)
				);
			}
		},
	});

	const form = useForm({
		defaultValues: ratesOf(settings),
		onSubmitMeta: { running: settings.running },
		onSubmit: ({ value, meta }) =>
			save.mutate({
				running: meta.running,
				deliveries_per_minute: value.deliveries ?? 0,
				orders_per_minute: value.orders ?? 0,
			}),
	});

	const stop = () =>
		save.mutate({
			running: false,
			deliveries_per_minute: settings.deliveries_per_minute,
			orders_per_minute: settings.orders_per_minute,
		});
	const pendingRunning = save.isPending ? save.variables.running : undefined;
	const fieldErrors = Object.values(serverErrors).some(Boolean);
	const generalError =
		save.error && !fieldErrors ? save.error.message : undefined;

	return (
		<Paper component='section' aria-labelledby='generator-title'>
			<PanelHeader>
				<Stack direction='row' sx={{ gap: 1.25, alignItems: 'center' }}>
					<Typography id='generator-title' variant='h3' component='h2'>
						Генератор событий
					</Typography>
					<StatusChip running={settings.running} disabled={disabled} />
				</Stack>
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					{settings.updated_at
						? `изменено ${formatDateTime(settings.updated_at)}`
						: 'на этой точке ещё не запускался'}
				</Typography>
			</PanelHeader>
			<Stack sx={{ p: 2.5, gap: 2.5 }}>
				{disabled && (
					<Alert severity='warning' icon={<Icon name='pointDisabled' />}>
						Точка отключена, поэтому генератор на ней ничего не создаёт. Его
						можно только остановить.
					</Alert>
				)}
				<Stack
					direction='row'
					sx={{
						gap: 2,
						alignItems: 'center',
						justifyContent: 'space-between',
						flexWrap: 'wrap',
					}}
				>
					<Typography sx={{ color: 'text.secondary' }}>
						{summary(settings, disabled)}
					</Typography>
					{settings.running ? (
						<Button
							variant='outlined'
							startIcon={<Icon name='stop' />}
							loading={pendingRunning === false}
							disabled={save.isPending}
							onClick={stop}
						>
							Остановить
						</Button>
					) : (
						!disabled && (
							<Button
								variant='contained'
								startIcon={<Icon name='play' />}
								loading={pendingRunning === true}
								disabled={save.isPending}
								onClick={() => void form.handleSubmit({ running: true })}
							>
								Запустить
							</Button>
						)
					)}
				</Stack>
				<Box
					sx={{
						display: 'grid',
						gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
						gap: 2,
					}}
				>
					{(['deliveries', 'orders'] as const).map((name) => (
						<form.Field
							key={name}
							name={name}
							validators={{ onChange: ({ value }) => rateError(value) }}
						>
							{(field) => {
								const error = field.state.meta.errors[0] ?? serverErrors[name];

								return (
									<QuantityField
										label={rateLabels[name]}
										fullWidth
										value={field.state.value}
										onValueChange={(value) => {
											setServerErrors((errors) => ({
												...errors,
												[name]: undefined,
											}));
											field.handleChange(value);
										}}
										format={{ maximumFractionDigits: 0 }}
										readOnly={disabled}
										error={Boolean(error)}
										helperText={error ?? rateHint}
									/>
								);
							}}
						</form.Field>
					))}
				</Box>
				{generalError && <Alert severity='error'>{generalError}</Alert>}
				{!disabled && (
					<form.Subscribe selector={(state) => state.isDefaultValue}>
						{(unchanged) => (
							<Stack
								direction='row'
								sx={{
									gap: 1.5,
									alignItems: 'center',
									justifyContent: 'flex-end',
									flexWrap: 'wrap',
								}}
							>
								<Typography variant='caption' sx={{ color: 'text.secondary' }}>
									Генератор подхватывает настройки за пару секунд
								</Typography>
								<Stack direction='row' sx={{ gap: 1 }}>
									{!unchanged && (
										<Button
											variant='text'
											disabled={save.isPending}
											onClick={() => {
												setServerErrors({});
												form.reset();
											}}
										>
											Вернуть как было
										</Button>
									)}
									<Button
										variant='outlined'
										disabled={unchanged || save.isPending}
										loading={pendingRunning === settings.running}
										onClick={() =>
											void form.handleSubmit({ running: settings.running })
										}
									>
										Сохранить
									</Button>
								</Stack>
							</Stack>
						)}
					</form.Subscribe>
				)}
			</Stack>
			<Stack
				component='ul'
				sx={{
					m: 0,
					px: 2.5,
					py: 2,
					gap: 1.25,
					listStyle: 'none',
					borderTop: '1px solid',
					borderColor: 'divider',
				}}
			>
				{hints.map((hint) => (
					<Stack
						key={hint.icon}
						component='li'
						direction='row'
						sx={{ gap: 1.25, color: 'text.secondary' }}
					>
						<Icon name={hint.icon} sx={{ color: 'text.tertiary' }} />
						<Typography variant='body2' sx={{ color: 'inherit' }}>
							{hint.text}
						</Typography>
					</Stack>
				))}
			</Stack>
		</Paper>
	);
}
