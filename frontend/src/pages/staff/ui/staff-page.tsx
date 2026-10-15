import {
	Box,
	Button,
	MenuItem,
	Paper,
	Select,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import {
	roleChoices,
	mayManageStaff,
	staffQueryOptions,
	type Staff,
} from '@entities/organization';
import { usePoint, writeAccess } from '@entities/profile';
import { roleLabels, type Role } from '@shared/auth';
import { countOf } from '@shared/lib/plural';
import { Avatar } from '@shared/ui/avatar';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { InfoNote } from '@shared/ui/info-note';
import { useNotice } from '@shared/ui/notice';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { ReadOnlyNote } from '@shared/ui/read-only-note';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import { useChangeStaffRole, useRemoveStaff } from '../api/staff-writes';
import { AddStaffDialog } from './add-staff-dialog';

interface Row {
	staff: Staff;
	me: boolean;
	/** Можно ли менять роль и снимать этого сотрудника. */
	manageable: boolean;
}

/** Общее для всех строк списка. */
interface ListProps {
	rows: Row[];
	owner: boolean;
	onRole: (staff: Staff, role: Role) => void;
	onRemove: (staff: Staff) => void;
}

type RowProps = Row & Omit<ListProps, 'rows'> & { touch?: boolean };

function Person({
	staff,
	me,
	touch,
}: Pick<RowProps, 'staff' | 'me' | 'touch'>) {
	return (
		<Stack direction='row' sx={{ alignItems: 'center', gap: 1.5, minWidth: 0 }}>
			<Avatar name={staff.name} size={touch ? 40 : 32} tone='neutral' />
			<Box sx={{ minWidth: 0 }}>
				<Typography sx={{ fontWeight: 500, color: 'text.strong' }} noWrap>
					{staff.name}
					{me && (
						<Box
							component='span'
							sx={{ fontWeight: 400, color: 'text.tertiary' }}
						>
							{' '}
							· это вы
						</Box>
					)}
				</Typography>
				<Typography
					variant='caption'
					component='div'
					sx={{ color: 'text.tertiary' }}
					noWrap
				>
					{staff.email}
				</Typography>
			</Box>
		</Stack>
	);
}

function RoleControl({ staff, manageable, owner, touch, onRole }: RowProps) {
	if (!manageable) {
		return <Typography>{roleLabels[staff.role]}</Typography>;
	}

	return (
		<Select
			size='small'
			value={staff.role}
			onChange={(event) => onRole(staff, event.target.value)}
			inputProps={{ 'aria-label': `Роль: ${staff.name}` }}
			sx={{ minWidth: 150, ...(touch && { minHeight: 44 }) }}
		>
			{roleChoices(owner).map(({ role, assignable }) => (
				<MenuItem key={role} value={role} disabled={!assignable}>
					<Box sx={{ flex: 1 }}>{roleLabels[role]}</Box>
					{!assignable && (
						<Typography
							variant='caption'
							sx={{ ml: 2, color: 'text.tertiary' }}
						>
							назначает владелец
						</Typography>
					)}
				</MenuItem>
			))}
		</Select>
	);
}

function RemoveButton({ staff, manageable, touch, onRemove }: RowProps) {
	if (!manageable) {
		return null;
	}

	return (
		<Button
			variant={touch ? 'outlined' : 'text'}
			size={touch ? 'large' : 'small'}
			startIcon={<Icon name='personRemove' />}
			aria-label={`Снять с точки: ${staff.name}`}
			onClick={() => onRemove(staff)}
			sx={touch ? { minHeight: 44 } : { color: 'text.secondary' }}
		>
			{touch ? 'Снять' : 'Снять с точки'}
		</Button>
	);
}

function StaffTable({ rows, ...list }: ListProps) {
	return (
		<Table aria-label='Сотрудники точки' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Сотрудник</TableCell>
					<TableCell sx={{ width: 240 }}>Роль</TableCell>
					<TableCell sx={{ width: 180 }} />
				</TableRow>
			</TableHead>
			<TableBody>
				{rows.map((row) => (
					<TableRow key={row.staff.user_id}>
						<TableCell>
							<Person {...row} />
						</TableCell>
						<TableCell>
							<RoleControl {...row} {...list} />
						</TableCell>
						<TableCell align='right'>
							<RemoveButton {...row} {...list} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function StaffCards({ rows, ...list }: ListProps) {
	return (
		<CardList label='Сотрудники точки'>
			{rows.map((row) => (
				<CardListItem key={row.staff.user_id} sx={{ flexWrap: 'wrap' }}>
					<Box sx={{ flex: '1 1 220px', minWidth: 0 }}>
						<Person {...row} touch />
					</Box>
					<Stack direction='row' sx={{ alignItems: 'center', gap: 1 }}>
						<RoleControl {...row} {...list} touch />
						<RemoveButton {...row} {...list} touch />
					</Stack>
				</CardListItem>
			))}
		</CardList>
	);
}

/** Раздел «Сотрудники»: кто назначен на точку; управляющий и владелец добавляют, меняют роль и снимают. */
export function StaffPage({ pointId }: { pointId: string }) {
	const wide = useWideLayout();
	const { profile, point } = usePoint(pointId);
	const owner = point.owner;
	const access = writeAccess(profile, point, 'staff.manage');
	const list = useQuery(staffQueryOptions(pointId));
	const changeRole = useChangeStaffRole(pointId);
	const remove = useRemoveStaff(pointId);
	const [adding, setAdding] = useState(false);
	const [removing, setRemoving] = useState<Staff | null>(null);
	const notice = useNotice();

	const onRole = (staff: Staff, role: Role) =>
		changeRole.mutate(
			{ userId: staff.user_id, role },
			{
				onSuccess: () =>
					notice.show(`${staff.name} теперь ${roleLabels[role].toLowerCase()}`),
				onError: (error) =>
					notice.show(`Не удалось сменить роль: ${error.message}`),
			}
		);

	const addButton = access === 'allowed' && (
		<Button
			variant='contained'
			startIcon={<Icon name='personAdd' />}
			onClick={() => setAdding(true)}
		>
			Добавить сотрудника
		</Button>
	);

	const content = (data: Staff[]) => {
		if (data.length === 0) {
			return (
				<EmptyState
					icon='staff'
					title='На точке пока нет сотрудников'
					description='Добавьте сотрудника по почте, с которой он входит в Goden. Владелец компании видит все точки и в список не входит.'
					action={addButton}
				/>
			);
		}

		const props: ListProps = {
			rows: data.map((staff) => ({
				staff,
				me: staff.user_id === profile.id,
				manageable: access === 'allowed' && mayManageStaff(owner, staff),
			})),
			owner,
			onRole,
			onRemove: setRemoving,
		};

		return wide ? <StaffTable {...props} /> : <StaffCards {...props} />;
	};

	return (
		<PageContent>
			<PageHeader
				title='Сотрудники'
				meta={
					list.data
						? `${point.name} · ${countOf(list.data.length, 'сотрудник', 'сотрудника', 'сотрудников')}`
						: point.name
				}
				actions={list.data?.length ? addButton : undefined}
			/>
			{access === 'forbidden' && (
				<Box sx={{ mb: 1.5 }}>
					<ReadOnlyNote>
						Добавлять, снимать и менять роль могут управляющий и владелец
					</ReadOnlyNote>
				</Box>
			)}
			<Paper sx={{ overflow: 'hidden' }}>
				<QueryState
					query={list}
					loading={
						<SkeletonList
							label='Загружаем сотрудников'
							count={5}
							height={44}
							sx={{ p: 2 }}
						/>
					}
					errorTitle='Не удалось загрузить сотрудников'
				>
					{content}
				</QueryState>
			</Paper>
			{access === 'allowed' && !owner && (
				<InfoNote sx={{ mt: 1.5 }}>
					Управляющих назначает и снимает владелец компании. Добавить можно
					только того, кто уже входил в Goden: попросите сотрудника
					зарегистрироваться и назвать вам почту.
				</InfoNote>
			)}
			{adding && (
				<AddStaffDialog
					pointId={pointId}
					owner={owner}
					onClose={() => setAdding(false)}
				/>
			)}
			{removing && (
				<ConfirmDialog
					open
					title={`Снять с точки: ${removing.name}`}
					description={`Сотрудник сразу потеряет доступ к точке «${point.name}». Добавить снова можно по почте ${removing.email}.`}
					confirmLabel='Снять с точки'
					confirmColor='error'
					pending={remove.isPending}
					error={remove.error?.message}
					onConfirm={() =>
						remove.mutate(removing.user_id, {
							onSuccess: () => setRemoving(null),
						})
					}
					onClose={() => {
						remove.reset();
						setRemoving(null);
					}}
				/>
			)}
			{notice.element}
		</PageContent>
	);
}
