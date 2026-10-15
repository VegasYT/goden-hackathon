import { Box, Button, Paper, Stack, Typography } from '@mui/material';
import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';

import { createCompany, createPoint } from '@entities/organization';
import {
	companyWithoutPoints,
	meQueryOptions,
	refreshProfile,
} from '@entities/profile';
import { logout } from '@shared/auth';
import { Icon } from '@shared/ui/icon';
import { Logo } from '@shared/ui/logo';

import { NameStep } from './name-step';

function EmployeeHint({ email }: { email: string }) {
	const { refetch, isFetching } = useSuspenseQuery(meQueryOptions);
	const navigate = useNavigate();
	const [notYet, setNotYet] = useState(false);

	const check = async () => {
		const { data } = await refetch();

		if (data && data.points.length > 0) {
			void navigate({ to: '/' });
			return;
		}

		setNotYet(true);
	};

	return (
		<Paper
			component='aside'
			aria-label='Если вы сотрудник'
			sx={{
				display: 'flex',
				gap: 1.5,
				width: '100%',
				maxWidth: 520,
				mt: 2,
				px: 2.5,
				py: 2,
			}}
		>
			<Icon name='employee' sx={{ mt: 0.25, color: 'text.secondary' }} />
			<Box sx={{ minWidth: 0 }}>
				<Typography>
					<Box component='span' sx={{ fontWeight: 500, color: 'text.strong' }}>
						Пришли работать в существующую компанию?
					</Box>{' '}
					Назовите владельцу или управляющему свою почту{' '}
					<Box
						component='span'
						sx={{ fontWeight: 500, color: 'text.strong', whiteSpace: 'nowrap' }}
					>
						{email}
					</Box>
					. Когда вас добавят на точку, нажмите «Проверить снова».
				</Typography>
				<Stack direction='row' sx={{ alignItems: 'center', gap: 1.5, mt: 1.5 }}>
					<Button
						variant='outlined'
						startIcon={<Icon name='refresh' />}
						loading={isFetching}
						onClick={() => void check()}
					>
						Проверить снова
					</Button>
					{notYet && (
						<Typography
							role='status'
							variant='body2'
							sx={{ color: 'text.secondary' }}
						>
							Вас пока не добавили на точку
						</Typography>
					)}
				</Stack>
			</Box>
		</Paper>
	);
}

/** Первый вход: создание компании и первой точки для пользователя без точек. */
export function WelcomePage() {
	const { data: profile } = useSuspenseQuery(meQueryOptions);
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	// Пока экран уходит на только что созданную точку, остаётся шаг этой точки.
	const company =
		profile.points.length > 0
			? profile.companies.find(({ id }) => id === profile.points[0].company_id)
			: companyWithoutPoints(profile);

	return (
		<Stack sx={{ minHeight: '100dvh', bgcolor: 'background.default' }}>
			<Box
				component='header'
				sx={{
					display: 'flex',
					alignItems: 'center',
					justifyContent: 'space-between',
					gap: 3,
					height: 64,
					px: 4,
					borderBottom: '1px solid',
					borderColor: 'divider',
				}}
			>
				<Logo />
				<Stack direction='row' sx={{ alignItems: 'center', gap: 1 }}>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{profile.email}
					</Typography>
					<Button
						startIcon={<Icon name='logout' />}
						onClick={() => void logout()}
					>
						Выйти
					</Button>
				</Stack>
			</Box>
			<Stack
				component='main'
				sx={{ alignItems: 'center', px: 2, pt: 12, pb: 4 }}
			>
				{company ? (
					<NameStep
						key={company.id}
						step={2}
						done={`Компания «${company.name}» создана`}
						title='Добавьте первую точку'
						description='Точка — это место со своим складом и кухней. Поставки, остатки, заказы и стоп-лист ведутся отдельно по каждой точке. Другие точки и сотрудников добавите позже.'
						label='Название точки'
						placeholder='Например, Центр'
						emptyError='Введите название точки'
						submitLabel='Создать точку'
						onSubmit={async (name, key) => {
							const point = await createPoint(company.id, name, key);

							await refreshProfile(queryClient);
							await navigate({
								to: '/points/$pointId',
								params: { pointId: point.id },
							});
						}}
					/>
				) : (
					<>
						<NameStep
							step={1}
							title='Создайте компанию'
							description='Компания объединяет ваши точки — у каждой свой склад и кухня. Каталог продуктов и блюд у компании общий.'
							label='Название компании'
							hint='Название видят сотрудники всех точек.'
							emptyError='Введите название компании'
							submitLabel='Продолжить'
							onSubmit={async (name, key) => {
								await createCompany(name, key);
								await refreshProfile(queryClient);
							}}
						/>
						<EmployeeHint email={profile.email} />
					</>
				)}
			</Stack>
		</Stack>
	);
}
