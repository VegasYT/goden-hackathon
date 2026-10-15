import {
	Alert,
	Button,
	Checkbox,
	FormControlLabel,
	IconButton,
	Paper,
	Stack,
	Typography,
} from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import {
	deliveryListQueryOptions,
	registeredMessage,
	registerDelivery,
	storeDelivery,
	type NewDelivery,
} from '@entities/delivery';
import { catalogProductsQueryOptions } from '@entities/product';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { countOf, plural } from '@shared/lib/plural';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { useNotice } from '@shared/ui/notice';
import { PanelHeader } from '@shared/ui/panel-header';
import { RetryAlert } from '@shared/ui/retry-alert';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { useWideLayout } from '@shared/ui/theme';

import {
	buildTestDelivery,
	testNumberPrefix,
	waitingTestDeliveries,
} from '../model/test-delivery';

const counts = ['1', '2', '3', '4', '5'] as const;

type Count = (typeof counts)[number];

interface Attempt {
	body: NewDelivery;
	key: string;
}

function ProblemOption({
	label,
	note,
	checked,
	disabled,
	onChange,
}: {
	label: string;
	note: string;
	checked: boolean;
	disabled: boolean;
	onChange: (checked: boolean) => void;
}) {
	return (
		<FormControlLabel
			disabled={disabled}
			sx={{ alignItems: 'flex-start', m: 0, gap: 1 }}
			control={
				<Checkbox
					checked={checked}
					onChange={(event) => onChange(event.target.checked)}
					sx={{ p: 0, mt: '1px' }}
				/>
			}
			label={
				<span>
					<Typography component='span' sx={{ color: 'text.strong' }}>
						{label}
					</Typography>
					<Typography
						variant='caption'
						component='span'
						sx={{ display: 'block', color: 'text.secondary' }}
					>
						{note}
					</Typography>
				</span>
			}
		/>
	);
}

function waitingText(count: number): string {
	if (count === 0) {
		return 'Тестовых поставок, ждущих приёмки, нет';
	}

	return `${plural(count, 'Ждёт', 'Ждут', 'Ждут')} приёмки ${countOf(count, 'тестовая поставка', 'тестовые поставки', 'тестовых поставок')}`;
}

interface TestDeliveryCardProps {
	pointId: string;
	companyId: string;
	/** Точка или её компания отключены: поставки на ней не регистрируются. */
	disabled: boolean;
}

/** Регистрирует поставку из случайных продуктов каталога и оставляет её для ручной приёмки. */
export function TestDeliveryCard({
	pointId,
	companyId,
	disabled,
}: TestDeliveryCardProps) {
	const queryClient = useQueryClient();
	const wide = useWideLayout();
	const products = useQuery(catalogProductsQueryOptions(companyId));
	const waiting = useQuery(
		deliveryListQueryOptions(pointId, waitingTestDeliveries)
	);
	const [count, setCount] = useState<Count>('3');
	const [expired, setExpired] = useState(false);
	const [shortLife, setShortLife] = useState(false);
	const notice = useNotice();
	const keyFor = useIdempotencyKey();

	const register = useMutation({
		mutationFn: ({ body, key }: Attempt) =>
			registerDelivery(pointId, body, key),
		onSuccess: (delivery) => {
			storeDelivery(queryClient, pointId, delivery);
			notice.show(
				registeredMessage(delivery.number, delivery.items.length),
				<>
					<ButtonLink
						to='/points/$pointId/deliveries/$deliveryId'
						params={{ pointId, deliveryId: delivery.id }}
					>
						Открыть
					</ButtonLink>
					<IconButton
						aria-label='Закрыть сообщение'
						color='inherit'
						onClick={notice.close}
					>
						<Icon name='close' />
					</IconButton>
				</>
			);
		},
	});

	const catalogEmpty = products.data?.total === 0;
	const unavailable = disabled || catalogEmpty || !products.data;

	const bring = () => {
		const body =
			products.data &&
			buildTestDelivery(
				products.data.items,
				{ count: Number(count), expired, shortLife },
				new Date()
			);

		if (body) {
			register.mutate({ body, key: keyFor(body) });
		}
	};

	return (
		<Paper component='section' aria-labelledby='test-delivery-title'>
			<PanelHeader>
				<Typography id='test-delivery-title' variant='h3' component='h2'>
					Тестовая поставка
				</Typography>
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					для ручной приёмки
				</Typography>
			</PanelHeader>
			<Stack sx={{ p: 2.5, gap: 2.5 }}>
				<Typography sx={{ color: 'text.secondary' }}>
					Регистрирует поставку из продуктов каталога и оставляет её без
					приёмки. Примите её вручную в разделе «Поставки».
				</Typography>
				{disabled && (
					<Alert severity='info' icon={<Icon name='pointDisabled' />}>
						На отключённой точке поставки не регистрируются.
					</Alert>
				)}
				{!disabled && catalogEmpty && (
					<Alert severity='info' icon={<Icon name='empty' />}>
						В каталоге компании нет продуктов. Запустите генератор: он заполнит
						каталог демонстрационным набором. Или добавьте продукты в каталог
						сами.
					</Alert>
				)}
				{products.isError && (
					<Alert severity='error'>{products.error.message}</Alert>
				)}
				<Stack sx={{ gap: 0.75, alignItems: wide ? 'flex-start' : 'stretch' }}>
					<Typography
						variant='body2'
						sx={{ fontWeight: 500, color: 'text.primary' }}
					>
						Позиций
					</Typography>
					<SegmentedControl<Count>
						label='Позиций'
						fullWidth={!wide}
						value={count}
						onChange={setCount}
						options={counts.map((value) => ({ value, label: value }))}
					/>
				</Stack>
				<Stack component='fieldset' sx={{ m: 0, p: 0, border: 0, gap: 1.5 }}>
					<Typography
						component='legend'
						variant='body2'
						sx={{ p: 0, mb: 1, fontWeight: 500, color: 'text.primary' }}
					>
						Добавить проблемные позиции
					</Typography>
					<ProblemOption
						label='Просроченную'
						note='срок годности истёк вчера, при приёмке её нужно отклонить'
						checked={expired}
						disabled={unavailable}
						onChange={setExpired}
					/>
					<ProblemOption
						label='С коротким сроком'
						note='срок истечёт через 10 минут, пока поставка ждёт приёмки'
						checked={shortLife}
						disabled={unavailable}
						onChange={setShortLife}
					/>
				</Stack>
				{register.isError && (
					<RetryAlert
						title='Поставку не привезли'
						error={register.error}
						note='Повтор не создаст вторую поставку.'
						retryDisabled={register.isPending}
						onRetry={() => register.mutate(register.variables)}
					/>
				)}
				<Stack direction='row' sx={{ gap: 1.5, alignItems: 'center' }}>
					<Button
						variant='contained'
						size='large'
						startIcon={<Icon name='deliveries' />}
						loading={register.isPending}
						disabled={unavailable}
						onClick={bring}
					>
						Привезти поставку
					</Button>
					<Typography variant='caption' sx={{ color: 'text.secondary' }}>
						номер начнётся с «{testNumberPrefix}»
					</Typography>
				</Stack>
			</Stack>
			{waiting.data && (
				<Stack
					direction='row'
					sx={{
						gap: 1.5,
						px: 2.5,
						py: 1,
						alignItems: 'center',
						justifyContent: 'space-between',
						flexWrap: 'wrap',
						borderTop: '1px solid',
						borderColor: 'divider',
					}}
				>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{waitingText(waiting.data.total)}
					</Typography>
					{waiting.data.total > 0 && (
						<ButtonLink
							to='/points/$pointId/deliveries'
							params={{ pointId }}
							search={{ status: 'registered', number: testNumberPrefix }}
							endIcon={<Icon name='arrowForward' />}
						>
							Открыть в «Поставках»
						</ButtonLink>
					)}
				</Stack>
			)}
			{notice.element}
		</Paper>
	);
}
