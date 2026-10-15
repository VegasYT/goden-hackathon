import {
	Alert,
	Box,
	Button,
	Checkbox,
	Dialog,
	DialogActions,
	DialogContent,
	DialogTitle,
	FormControl,
	FormControlLabel,
	FormGroup,
	FormHelperText,
	FormLabel,
	InputLabel,
	Stack,
	TextField,
	Typography,
} from '@mui/material';
import { useId, useRef, useState } from 'react';

import type { Company, Point } from '@entities/organization';
import { ApiError } from '@shared/api';
import { FormDialog } from '@shared/ui/form-dialog';
import { Icon } from '@shared/ui/icon';

import {
	useCreateSupplier,
	useUpdateSupplier,
	type NewSupplier,
	type Supplier,
} from '../api/suppliers';

interface SupplierDialogProps {
	company: Company;
	/** Работающие точки компании: из них выбираются точки поставщика. */
	points: Point[];
	/** Поставщик для изменения; без него диалог заводит нового. */
	supplier?: Supplier;
	onSaved: (saved: Supplier) => void;
	onClose: () => void;
}

/** Новый поставщик или изменение поставщика: название и точки, на которые он возит. */
export function SupplierDialog({
	company,
	points,
	supplier,
	onSaved,
	onClose,
}: SupplierDialogProps) {
	const [name, setName] = useState(supplier?.name ?? '');
	const [pointIds, setPointIds] = useState<string[]>(supplier?.point_ids ?? []);
	const [tried, setTried] = useState(false);
	const create = useCreateSupplier(company.id);
	const update = useUpdateSupplier(company.id, supplier?.id ?? '');
	const mutation = supplier ? update : create;
	const value = name.trim();
	const chosen = pointIds.filter((id) => points.some((p) => p.id === id));
	const fields =
		mutation.error instanceof ApiError ? mutation.error.fieldErrors : {};
	const nameError =
		tried && !value ? 'Введите название поставщика' : fields.name?.message;
	const pointsError =
		tried && chosen.length === 0
			? 'Отметьте хотя бы одну точку'
			: fields.point_ids?.message;
	const otherError =
		!fields.name && !fields.point_ids ? mutation.error?.message : undefined;

	const toggle = (id: string, on: boolean) =>
		setPointIds((current) =>
			on ? [...current, id] : current.filter((other) => other !== id)
		);

	return (
		<FormDialog
			title={supplier ? `Поставщик «${supplier.name}»` : 'Новый поставщик'}
			submitLabel={supplier ? 'Сохранить' : 'Создать'}
			pending={mutation.isPending}
			error={otherError}
			onSubmit={() => {
				setTried(true);

				if (value && chosen.length > 0) {
					const input: NewSupplier = { name: value, point_ids: chosen };

					mutation.mutate(input, { onSuccess: onSaved });
				}
			}}
			onClose={onClose}
		>
			<TextField
				label='Название поставщика'
				placeholder='Например, Овощебаза Юг'
				autoFocus
				value={name}
				onChange={(event) => setName(event.target.value)}
				error={!!nameError}
				helperText={nameError || 'Его видят сотрудники в истории поставок'}
			/>
			<FormControl error={!!pointsError} component='fieldset'>
				<FormLabel component='legend'>Точки, на которые он возит</FormLabel>
				<FormGroup>
					{points.map((point) => (
						<FormControlLabel
							key={point.id}
							label={point.name}
							control={
								<Checkbox
									checked={pointIds.includes(point.id)}
									onChange={(event) => toggle(point.id, event.target.checked)}
								/>
							}
						/>
					))}
				</FormGroup>
				<FormHelperText>
					{pointsError ||
						'На других точках поставщик поставки не зарегистрирует'}
				</FormHelperText>
			</FormControl>
		</FormDialog>
	);
}

function CopyField({ label, value }: { label: string; value: string }) {
	const id = useId();
	const input = useRef<HTMLInputElement>(null);
	const [copied, setCopied] = useState(false);
	// Без буфера обмена (страница не по https) выделяем текст, чтобы его скопировали сами.
	const select = () => input.current?.select();

	const copy = () => {
		if (!navigator.clipboard) {
			select();
			return;
		}

		navigator.clipboard.writeText(value).then(() => setCopied(true), select);
	};

	return (
		<Box>
			<InputLabel htmlFor={id} shrink sx={{ whiteSpace: 'normal' }}>
				{label}
			</InputLabel>
			<Stack direction='row' sx={{ gap: 1, alignItems: 'center' }}>
				<TextField
					id={id}
					value={value}
					fullWidth
					inputRef={input}
					slotProps={{
						htmlInput: {
							readOnly: true,
							sx: { fontVariantNumeric: 'tabular-nums' },
						},
					}}
				/>
				<Button
					variant='outlined'
					startIcon={<Icon name={copied ? 'check' : 'copy'} />}
					onClick={copy}
					sx={{ flexShrink: 0, minWidth: 156 }}
				>
					{copied ? 'Скопировано' : 'Скопировать'}
				</Button>
			</Stack>
		</Box>
	);
}

interface KeyDialogProps {
	company: Company;
	points: Point[];
	/** Ответ на создание или выпуск ключа: только в нём есть ключ. */
	supplier: Supplier;
	onClose: () => void;
}

/** Ключ поставщика и данные для подключения; ключ показывается один раз, поэтому окно закрывает только «Готово». */
export function KeyDialog({
	company,
	points,
	supplier,
	onClose,
}: KeyDialogProps) {
	const own = points.filter(({ id }) => supplier.point_ids.includes(id));

	return (
		<Dialog open maxWidth='sm' fullWidth>
			<DialogTitle>Ключ поставщика «{supplier.name}»</DialogTitle>
			<DialogContent>
				<Stack sx={{ gap: 2.5, pt: 0.5 }}>
					<Alert severity='warning'>
						Скопируйте ключ и передайте поставщику. После закрытия окна его
						больше не показать: если ключ потеряется, выпустите новый.
					</Alert>
					<CopyField label='Ключ' value={supplier.key ?? ''} />
					<Box>
						<Typography variant='h3'>Данные для подключения</Typography>
						<Typography sx={{ color: 'text.secondary', mb: 1 }}>
							Ключ передаётся в заголовке Authorization: Bearer. Поставщик
							регистрирует поставки через {window.location.origin}/api/v1.
						</Typography>
						<Stack sx={{ gap: 1.5 }}>
							<CopyField
								label={`Компания «${company.name}»`}
								value={company.id}
							/>
							{own.map((point) => (
								<CopyField
									key={point.id}
									label={`Точка «${point.name}»`}
									value={point.id}
								/>
							))}
						</Stack>
					</Box>
				</Stack>
			</DialogContent>
			<DialogActions>
				<Button variant='contained' onClick={onClose}>
					Готово
				</Button>
			</DialogActions>
		</Dialog>
	);
}
