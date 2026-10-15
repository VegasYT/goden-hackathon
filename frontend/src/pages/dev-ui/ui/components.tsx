import {
	Alert,
	Button,
	Chip,
	IconButton,
	MenuItem,
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
import { useState } from 'react';

import {
	formatAmount,
	formatAmountDelta,
	formatAmountOf,
} from '@shared/lib/quantity';
import { BreadcrumbLink, Breadcrumbs } from '@shared/ui/breadcrumbs';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import {
	ConnectionBanner,
	ConnectionIndicator,
} from '@shared/ui/connection-status';
import { EmptyState, NoResultsState } from '@shared/ui/empty-state';
import { EmptyValue } from '@shared/ui/empty-value';
import { DisclosureIcon, Icon } from '@shared/ui/icon';
import { QuantityField } from '@shared/ui/quantity-field';
import { RetryAlert } from '@shared/ui/retry-alert';
import { DateField, DateTimeField } from '@shared/ui/date-field';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { SidePanel } from '@shared/ui/side-panel';

import { Section } from './section';

const sampleTime = new Date(2026, 9, 10, 14, 32);
const noop = () => {};
const sampleError = new Error('Нет связи с сервером');

export function ButtonsSection() {
	return (
		<Section
			title='Кнопки'
			description='Одна основная кнопка на экран. Без капса и ряби, фокус — зелёное кольцо.'
		>
			<Stack
				direction='row'
				sx={{ gap: 1, flexWrap: 'wrap', alignItems: 'center' }}
			>
				<Button variant='contained'>Подтвердить приёмку</Button>
				<Button variant='outlined'>Открытый карантин</Button>
				<Button>История событий</Button>
				<Button variant='contained' color='error'>
					Отклонить окончательно
				</Button>
				<Button variant='contained' disabled>
					Недоступно
				</Button>
				<Button variant='outlined' size='small'>
					Маленькая
				</Button>
				<Button variant='contained' size='large'>
					Крупная
				</Button>
				<IconButton aria-label='Действия строки'>
					<Icon name='more' />
				</IconButton>
			</Stack>
		</Section>
	);
}

export function FieldsSection() {
	const [filter, setFilter] = useState<'all' | 'pending' | 'expired'>('all');
	const [expiresAt, setExpiresAt] = useState<Date | null>(
		new Date(2026, 9, 17, 23, 59)
	);
	const [madeAt, setMadeAt] = useState<Date | null>(null);

	return (
		<Section
			title='Поля'
			description='Подпись над полем, граница с контрастом не ниже 3:1.'
		>
			<Stack
				direction='row'
				sx={{ gap: 2, flexWrap: 'wrap', alignItems: 'flex-start', mb: 2.5 }}
			>
				<TextField label='Комментарий' placeholder='2 пакета вздуты' />
				<TextField
					label='Причина'
					select
					defaultValue='package'
					sx={{ minWidth: 220 }}
				>
					<MenuItem value='package'>Повреждена упаковка</MenuItem>
					<MenuItem value='temperature'>Нарушен температурный режим</MenuItem>
				</TextField>
				<TextField
					label='Номер партии'
					error
					helperText='Такой партии нет'
					defaultValue='П-9999'
				/>
			</Stack>
			<Stack
				direction='row'
				sx={{ gap: 2, flexWrap: 'wrap', alignItems: 'flex-start', mb: 2.5 }}
			>
				<QuantityField label='Принять' unit='кг' defaultValue={12} step={0.5} />
				<QuantityField
					label='Отклонить'
					unit='л'
					defaultValue={2.5}
					step={0.5}
				/>
				<QuantityField label='Карантин' unit='кг' defaultValue={0} disabled />
			</Stack>
			<Typography variant='body2' sx={{ mb: 2.5, color: 'text.secondary' }}>
				Количества на складе: {formatAmount(180, 'g')} ·{' '}
				{formatAmount(2450, 'g')} · {formatAmountOf(1600, 2000, 'g')} ·{' '}
				{formatAmountDelta(-200, 'g')} · {formatAmount(24, 'pcs')}
			</Typography>
			<Stack
				direction='row'
				sx={{ gap: 2, flexWrap: 'wrap', alignItems: 'flex-start', mb: 2.5 }}
			>
				<DateTimeField
					label='Годен до'
					value={expiresAt}
					onChange={setExpiresAt}
				/>
				<DateField
					label='Изготовлено'
					value={madeAt}
					onChange={setMadeAt}
					error
					helperText='Позже срока годности'
				/>
			</Stack>
			<SegmentedControl
				label='Фильтр позиций'
				value={filter}
				onChange={setFilter}
				options={[
					{ value: 'all', label: 'Все', count: 8 },
					{ value: 'pending', label: 'Требуют решения', count: 1 },
					{ value: 'expired', label: 'Просрочены', count: 1 },
				]}
			/>
		</Section>
	);
}

export function StatusSection() {
	return (
		<Section
			title='Статусы'
			description='Янтарный — внимание, красный — опасность, зелёный — успех. Нормальное состояние без цвета.'
		>
			<Stack direction='row' sx={{ gap: 1, flexWrap: 'wrap', mb: 2 }}>
				<Chip label='ожидает приёмки' />
				<Chip label='ещё 2 дня' color='warning' />
				<Chip label='просрочен' color='error' />
				<Chip label='принято' color='success' />
			</Stack>
			<Stack sx={{ gap: 1 }}>
				<Alert severity='warning'>Пармезан ещё без решения</Alert>
				<Alert severity='error'>Тирамису в стоп-листе: нет маскарпоне</Alert>
				<Alert severity='success'>Поставка № 118 принята</Alert>
			</Stack>
		</Section>
	);
}

const rows = [
	{
		name: 'Лосось охлаждённый',
		qty: '12,0 кг',
		until: '14.10, 09:00',
		status: null,
	},
	{
		name: 'Томаты черри',
		qty: '2,5 кг',
		until: '13.10, 18:00',
		status: 'warning' as const,
	},
	{
		name: 'Шпинат свежий',
		qty: '3,0 кг',
		until: '09.10, 22:00',
		status: 'error' as const,
	},
	{
		name: 'Яйца куриные С0',
		qty: '360 шт',
		until: '30.10, 23:59',
		status: null,
	},
];

export function TableSection() {
	return (
		<Section
			title='Таблица'
			description='Заголовки без капса, числа выровнены вправо табличными цифрами.'
		>
			<Paper>
				<Table>
					<TableHead>
						<TableRow>
							<TableCell>Продукт</TableCell>
							<TableCell align='right'>Количество</TableCell>
							<TableCell>Годен до</TableCell>
							<TableCell>Статус</TableCell>
						</TableRow>
					</TableHead>
					<TableBody>
						{rows.map((row) => (
							<TableRow key={row.name}>
								<TableCell sx={{ color: 'text.strong', fontWeight: 500 }}>
									{row.name}
								</TableCell>
								<TableCell align='right'>{row.qty}</TableCell>
								<TableCell>{row.until}</TableCell>
								<TableCell>
									{row.status === 'warning' && (
										<Chip label='ещё 2 дня' color='warning' />
									)}
									{row.status === 'error' && (
										<Chip label='просрочен' color='error' />
									)}
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</Paper>
		</Section>
	);
}

export function FeedbackSection() {
	const [open, setOpen] = useState(false);
	const [failed, setFailed] = useState(false);
	const [pending, setPending] = useState(false);
	const [panelOpen, setPanelOpen] = useState(false);

	const retry = () => {
		setPending(true);
		window.setTimeout(() => setPending(false), 1500);
	};

	return (
		<Section title='Диалог и пустое состояние'>
			<Stack direction='row' sx={{ gap: 1, flexWrap: 'wrap' }}>
				<Button variant='outlined' onClick={() => setOpen(true)}>
					Открыть диалог подтверждения
				</Button>
				<Button variant='outlined' onClick={() => setFailed(true)}>
					Диалог: отправка и ошибка
				</Button>
				<Button variant='outlined' onClick={() => setPanelOpen(true)}>
					Открыть боковую панель
				</Button>
			</Stack>
			<SidePanel
				open={panelOpen}
				title='Открытый карантин'
				count={2}
				closeLabel='Закрыть карантин'
				onClose={() => setPanelOpen(false)}
				width={440}
			>
				<Typography sx={{ p: 3, color: 'text.secondary' }}>
					Позиции из прошлых поставок ждут решения. На планшете панель занимает
					весь экран.
				</Typography>
			</SidePanel>
			<ConfirmDialog
				open={failed}
				title='Подтвердить приёмку поставки № 118?'
				confirmLabel='Повторить'
				onConfirm={retry}
				onClose={() => setFailed(false)}
				pending={pending}
				error='Нет связи с сервером. Проверьте подключение и повторите'
			/>
			<ConfirmDialog
				open={open}
				title='Подтвердить приёмку поставки № 118?'
				description='Проверьте итог. На склад попадёт только принятое количество.'
				confirmLabel='Подтвердить приёмку'
				onConfirm={() => setOpen(false)}
				onClose={() => setOpen(false)}
				dismissibleNotice={{
					id: 'dev-ui-approval',
					text: 'После подтверждения приёмку нельзя изменить — только исправить отдельным действием с указанием причины.',
				}}
			>
				<Typography>
					На склад · 7 позиций, отклонено · 2, в карантин · 1
				</Typography>
			</ConfirmDialog>
			<EmptyState
				title='Поставок пока нет'
				description='Новые поставки появятся здесь, как только их зарегистрируют.'
				action={<Button variant='outlined'>Запустить генератор</Button>}
				titleComponent='h3'
			/>
		</Section>
	);
}

export function ConnectionSection() {
	return (
		<Section
			title='Связь с сервером'
			description='Индикатор стоит в навигации точки, плашка появляется над страницей, когда связи нет.'
		>
			<Stack sx={{ gap: 2 }}>
				<Stack direction='row' sx={{ gap: 3, alignItems: 'center' }}>
					<ConnectionIndicator
						state={{ status: 'online', updatedAt: sampleTime }}
					/>
					<ConnectionIndicator
						state={{ status: 'online', updatedAt: sampleTime }}
						compact
					/>
					<ConnectionIndicator
						state={{ status: 'offline', dataAt: sampleTime, onRetry: noop }}
						compact
					/>
				</Stack>
				<ConnectionBanner
					state={{ status: 'offline', dataAt: sampleTime, onRetry: noop }}
				/>
			</Stack>
		</Section>
	);
}

export function ListPartsSection() {
	const [open, setOpen] = useState(false);

	return (
		<Section
			title='Списки и состояния'
			description='Путь страницы, карточки планшета, пустое значение, раскрытие строки, пустой поиск и ошибка записи.'
		>
			<Stack sx={{ gap: 2 }}>
				<Breadcrumbs current='Новая поставка'>
					<BreadcrumbLink to='/dev/ui'>Поставки</BreadcrumbLink>
				</Breadcrumbs>
				<Paper>
					<CardList label='Пример карточек'>
						<CardListItem>
							<IconButton
								aria-label='Раскрыть поставку'
								aria-expanded={open}
								onClick={() => setOpen(!open)}
							>
								<DisclosureIcon open={open} />
							</IconButton>
							<Typography sx={{ flex: 1 }}>Поставка № 118</Typography>
							<Chip label='ждёт приёмки' />
						</CardListItem>
						<CardListItem>
							<Typography sx={{ flex: 1 }}>Молоко 3,2%</Typography>
							<EmptyValue />
						</CardListItem>
					</CardList>
					<NoResultsState
						description='Нет продуктов с «сыр» в названии'
						onReset={noop}
					/>
				</Paper>
				<RetryAlert
					title='Поставку не привезли'
					error={sampleError}
					note='Повтор не создаст вторую поставку.'
					onRetry={noop}
				/>
			</Stack>
		</Section>
	);
}
