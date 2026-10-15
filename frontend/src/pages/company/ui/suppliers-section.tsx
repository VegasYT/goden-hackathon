import {
	Box,
	Button,
	Chip,
	Paper,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import type { Company, Point } from '@entities/organization';
import { ActionMenu, type ActionMenuItem } from '@shared/ui/action-menu';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { SkeletonList } from '@shared/ui/skeleton-list';

import {
	suppliersQueryOptions,
	useIssueSupplierKey,
	useRevokeSupplierKey,
	type Supplier,
} from '../api/suppliers';
import { KeyDialog, SupplierDialog } from './supplier-dialogs';

type Open =
	| { kind: 'new' }
	| { kind: 'edit' | 'issue' | 'revoke' | 'key'; supplier: Supplier }
	| null;

const explanation =
	'Поставщик с ключом сам регистрирует поставки на своих точках, а принимает их приёмщик.';

function pointNames(supplier: Supplier, points: Point[]) {
	return points
		.filter(({ id }) => supplier.point_ids.includes(id))
		.map(({ name }) => name)
		.join(', ');
}

/** Поставщики компании и их ключи на «Точках и компании»; у отключённой компании только чтение. */
export function SuppliersSection({
	company,
	points,
}: {
	company: Company;
	/** Все точки компании, включая отключённые. */
	points: Point[];
}) {
	const suppliers = useQuery(suppliersQueryOptions(company.id));
	const issue = useIssueSupplierKey(company.id);
	const revoke = useRevokeSupplierKey(company.id);
	const [open, setOpen] = useState<Open>(null);
	const editable = company.active;
	const working = points.filter(({ active }) => active);
	const current =
		open?.kind === 'issue' || open?.kind === 'revoke'
			? open.supplier
			: undefined;
	// Подтверждение закрывается с анимацией: до её конца в нём остаётся прежний поставщик.
	const [shown, setShown] = useState<Supplier>();

	if (current && current !== shown) {
		setShown(current);
	}

	const target = current ?? shown;
	const close = () => {
		issue.reset();
		revoke.reset();
		setOpen(null);
	};

	const actions = (supplier: Supplier): ActionMenuItem[] => [
		{
			icon: 'edit',
			label: 'Изменить',
			onClick: () => setOpen({ kind: 'edit', supplier }),
		},
		{
			icon: 'key',
			label: supplier.key_active ? 'Перевыпустить ключ' : 'Выпустить ключ',
			onClick: () => setOpen({ kind: 'issue', supplier }),
		},
		...(supplier.key_active
			? [
					{
						icon: 'close',
						label: 'Отозвать ключ',
						danger: true,
						onClick: () => setOpen({ kind: 'revoke', supplier }),
					} satisfies ActionMenuItem,
				]
			: []),
	];

	return (
		<Paper
			component='section'
			aria-label='Поставщики'
			sx={{ overflow: 'hidden' }}
		>
			<Box
				sx={{
					px: 2,
					pt: 2,
					pb: 1,
					display: 'flex',
					gap: 2,
					alignItems: 'flex-start',
				}}
			>
				<Box sx={{ flex: 1 }}>
					<Typography variant='h2'>Поставщики</Typography>
					<Typography sx={{ color: 'text.secondary', mt: 0.5 }}>
						{explanation}
					</Typography>
				</Box>
				{editable && (
					<Button
						variant='outlined'
						startIcon={<Icon name='add' />}
						onClick={() => setOpen({ kind: 'new' })}
						sx={{ flexShrink: 0 }}
					>
						Новый поставщик
					</Button>
				)}
			</Box>
			<QueryState
				query={suppliers}
				loading={
					<SkeletonList
						label='Загружаем поставщиков'
						count={2}
						height={40}
						sx={{ p: 2 }}
					/>
				}
				errorTitle='Не удалось загрузить поставщиков'
			>
				{(data) =>
					data.length === 0 ? (
						<EmptyState
							icon='key'
							title='Поставщиков пока нет'
							titleComponent='h3'
						/>
					) : (
						<Table
							aria-label='Поставщики компании'
							sx={{ tableLayout: 'fixed' }}
						>
							<TableHead>
								<TableRow>
									<TableCell>Поставщик</TableCell>
									<TableCell>Точки</TableCell>
									<TableCell sx={{ width: 150 }}>Ключ</TableCell>
									{editable && <TableCell sx={{ width: 64 }} />}
								</TableRow>
							</TableHead>
							<TableBody>
								{data.map((supplier) => (
									<TableRow key={supplier.id}>
										<TableCell sx={{ fontWeight: 500 }}>
											{supplier.name}
										</TableCell>
										<TableCell sx={{ color: 'text.secondary' }}>
											{pointNames(supplier, points)}
										</TableCell>
										<TableCell>
											{supplier.key_active ? (
												<Chip color='success' label='ключ действует' />
											) : (
												<Chip label='ключ отозван' />
											)}
										</TableCell>
										{editable && (
											<TableCell align='right'>
												<ActionMenu
													label={`Действия: ${supplier.name}`}
													items={actions(supplier)}
												/>
											</TableCell>
										)}
									</TableRow>
								))}
							</TableBody>
						</Table>
					)
				}
			</QueryState>
			{(open?.kind === 'new' || open?.kind === 'edit') && (
				<SupplierDialog
					company={company}
					points={working}
					supplier={open.kind === 'edit' ? open.supplier : undefined}
					onSaved={(saved) =>
						setOpen(saved.key ? { kind: 'key', supplier: saved } : null)
					}
					onClose={() => setOpen(null)}
				/>
			)}
			{open?.kind === 'key' && (
				<KeyDialog
					company={company}
					points={points}
					supplier={open.supplier}
					onClose={() => setOpen(null)}
				/>
			)}
			<ConfirmDialog
				open={open?.kind === 'issue'}
				title={
					target?.key_active
						? `Перевыпустить ключ поставщика «${target.name}»?`
						: `Выпустить ключ поставщику «${target?.name}»?`
				}
				description={
					target?.key_active
						? 'Прежний ключ сразу перестанет действовать. Передайте поставщику новый.'
						: 'С новым ключом поставщик снова сможет регистрировать поставки.'
				}
				confirmLabel={
					target?.key_active ? 'Перевыпустить ключ' : 'Выпустить ключ'
				}
				pending={issue.isPending}
				error={issue.error?.message}
				onConfirm={() =>
					target &&
					issue.mutate(target.id, {
						onSuccess: (saved) => setOpen({ kind: 'key', supplier: saved }),
					})
				}
				onClose={close}
			/>
			<ConfirmDialog
				open={open?.kind === 'revoke'}
				title={`Отозвать ключ поставщика «${target?.name}»?`}
				description='Поставщик сразу потеряет доступ и не сможет регистрировать поставки.'
				confirmLabel='Отозвать ключ'
				confirmColor='error'
				pending={revoke.isPending}
				error={revoke.error?.message}
				onConfirm={() =>
					target && revoke.mutate(target.id, { onSuccess: close })
				}
				onClose={close}
			/>
		</Paper>
	);
}
