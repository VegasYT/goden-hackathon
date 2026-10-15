import {
	Box,
	Button,
	Chip,
	Paper,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';

import {
	companyPointsQueryOptions,
	type Company,
	type Point,
} from '@entities/organization';
import { usePoint } from '@entities/profile';
import { QueryState } from '@shared/ui/empty-state';
import { EmptyValue } from '@shared/ui/empty-value';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { useNotice } from '@shared/ui/notice';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { ReadOnlyNote } from '@shared/ui/read-only-note';
import { RenameForm } from '@shared/ui/rename-form';
import { SettingsSection } from '@shared/ui/settings-section';
import { SkeletonList } from '@shared/ui/skeleton-list';

import { useRenameCompany } from '../api/company-writes';
import {
	disableWarning,
	DisableCompanyDialog,
	NewPointDialog,
	TransferDialog,
	transferWarning,
} from './company-dialogs';
import { SuppliersSection } from './suppliers-section';

type OpenDialog = 'newPoint' | 'transfer' | 'disable' | null;

function pointsSummary(points: Point[], company: Company) {
	const working = company.active
		? points.filter(({ active }) => active).length
		: 0;
	const disabled = points.length - working;

	return disabled > 0
		? `Компания «${company.name}» · работают: ${working}, отключены: ${disabled}`
		: `Компания «${company.name}» · точек: ${points.length}`;
}

function PointsTable({
	points,
	company,
}: {
	points: Point[];
	company: Company;
}) {
	return (
		<Table aria-label='Точки компании' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Точка</TableCell>
					<TableCell sx={{ width: 130 }}>Состояние</TableCell>
					<TableCell sx={{ width: 150 }}>Telegram</TableCell>
					<TableCell sx={{ width: 170 }} />
				</TableRow>
			</TableHead>
			<TableBody>
				{points.map((point) => {
					const working = point.active && company.active;

					return (
						<TableRow key={point.id}>
							<TableCell sx={{ fontWeight: 500 }}>{point.name}</TableCell>
							<TableCell>
								{working ? (
									<Chip color='success' label='работает' />
								) : (
									<Chip label='отключена' />
								)}
							</TableCell>
							<TableCell sx={{ color: 'text.secondary' }}>
								{!working ? (
									<EmptyValue />
								) : point.telegram_chat_id === undefined ? (
									'не подключены'
								) : (
									'подключены'
								)}
							</TableCell>
							<TableCell align='right'>
								{working ? (
									<ButtonLink
										variant='outlined'
										size='small'
										to='/points/$pointId/settings'
										params={{ pointId: point.id }}
										aria-label={`Настройки: ${point.name}`}
									>
										Настройки
									</ButtonLink>
								) : (
									<ButtonLink
										size='small'
										to='/points/$pointId'
										params={{ pointId: point.id }}
										aria-label={`Открыть: ${point.name}`}
									>
										Открыть
									</ButtonLink>
								)}
							</TableCell>
						</TableRow>
					);
				})}
			</TableBody>
		</Table>
	);
}

/** «Точки и компания»: компания выбранной точки, её точки, поставщики, название, передача и отключение; только владелец. */
export function CompanyPage({ pointId }: { pointId: string }) {
	const navigate = useNavigate();
	const { profile, point } = usePoint(pointId);
	const companyId = point.company_id;
	const company = profile.companies.find(({ id }) => id === companyId);
	const points = useQuery({
		...companyPointsQueryOptions(companyId),
		enabled: !!company,
	});
	const rename = useRenameCompany(companyId);
	const [dialog, setDialog] = useState<OpenDialog>(null);
	const notice = useNotice();

	if (!company) {
		return null;
	}

	const editable = company.active;

	return (
		<PageContent>
			<PageHeader
				title='Точки и компания'
				meta={
					points.data
						? pointsSummary(points.data, company)
						: `Компания «${company.name}»`
				}
				actions={
					editable && (
						<Button
							variant='contained'
							startIcon={<Icon name='add' />}
							onClick={() => setDialog('newPoint')}
						>
							Новая точка
						</Button>
					)
				}
			/>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: {
						xs: 'minmax(0, 1fr)',
						lg: 'minmax(0, 1fr) 380px',
					},
					gap: 2,
					alignItems: 'start',
				}}
			>
				<Stack sx={{ gap: 2 }}>
					<Paper
						component='section'
						aria-label='Точки'
						sx={{ overflow: 'hidden' }}
					>
						<Box sx={{ px: 2, pt: 2, pb: 1 }}>
							<Typography variant='h2'>Точки</Typography>
							<Typography sx={{ color: 'text.secondary', mt: 0.5 }}>
								У каждой точки свой склад, кухня и сотрудники. Каталог общий.
							</Typography>
						</Box>
						<QueryState
							query={points}
							loading={
								<SkeletonList
									label='Загружаем точки'
									count={3}
									height={40}
									sx={{ p: 2 }}
								/>
							}
							errorTitle='Не удалось загрузить точки'
						>
							{(data) => <PointsTable points={data} company={company} />}
						</QueryState>
					</Paper>
					{points.data && (
						<SuppliersSection company={company} points={points.data} />
					)}
				</Stack>
				<Stack sx={{ gap: 2 }}>
					<SettingsSection title='Компания'>
						<RenameForm
							label='Название'
							value={company.name}
							hint='Видят сотрудники всех точек'
							pending={rename.isPending}
							error={rename.error?.message}
							onSave={(name) => rename.mutate(name)}
							readOnlyNote={
								!editable && (
									<ReadOnlyNote>
										Компания отключена — менять её нельзя
									</ReadOnlyNote>
								)
							}
						/>
					</SettingsSection>
					{editable && (
						<>
							<SettingsSection
								title='Передать компанию'
								description={transferWarning}
								action={
									<Button
										variant='outlined'
										startIcon={<Icon name='transfer' />}
										onClick={() => setDialog('transfer')}
									>
										Передать компанию
									</Button>
								}
							/>
							<SettingsSection
								danger
								title='Отключить компанию'
								description={disableWarning}
								action={
									<Button
										variant='outlined'
										color='error'
										onClick={() => setDialog('disable')}
									>
										Отключить компанию
									</Button>
								}
							/>
						</>
					)}
				</Stack>
			</Box>
			{dialog === 'newPoint' && (
				<NewPointDialog
					company={company}
					onCreated={(created) => {
						setDialog(null);
						notice.show(
							`Точка «${created.name}» создана`,
							<ButtonLink
								to='/points/$pointId'
								params={{ pointId: created.id }}
							>
								Открыть
							</ButtonLink>
						);
					}}
					onClose={() => setDialog(null)}
				/>
			)}
			{dialog === 'transfer' && (
				<TransferDialog
					company={company}
					onDone={() => void navigate({ to: '/' })}
					onClose={() => setDialog(null)}
				/>
			)}
			{dialog === 'disable' && (
				<DisableCompanyDialog
					company={company}
					onDone={() => setDialog(null)}
					onClose={() => setDialog(null)}
				/>
			)}
			{notice.element}
		</PageContent>
	);
}
