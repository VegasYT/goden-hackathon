import {
	Autocomplete,
	Box,
	InputAdornment,
	TextField,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { quantityInput } from '@shared/lib/quantity';
import { searchDelayMs, searchTextMaxLength } from '@shared/lib/search-params';
import { Icon } from '@shared/ui/icon';

import { productListQueryOptions, type Product } from '../api/product-queries';
import { productNote } from '../model/product';

const optionLimit = 20;

interface CreateOption {
	create: string;
}

type Option = Product | CreateOption;

const isCreate = (option: Option): option is CreateOption => 'create' in option;

interface ProductPickerProps {
	companyId: string;
	value: Product | null;
	/** Поле добавляет продукты в список: после выбора оно очищается. */
	clearOnSelect?: boolean;
	onChange: (product: Product | null) => void;
	/** Причина, по которой продукт выбрать нельзя; `undefined` — можно. */
	disabledReason?: (product: Product) => string | undefined;
	/** Пункт «Новый продукт «<текст>»» в конце списка. */
	onCreate?: (name: string) => void;
	label?: string;
	/** Подпись для программ чтения с экрана, когда видимой подписи нет. */
	ariaLabel?: string;
	placeholder?: string;
	error?: string;
	/** Подпись единицы в списке; по умолчанию единица ввода поставки («кг», «л», «шт»). */
	unitLabel?: (unit: Product['unit']) => string;
}

/** Выбор действующего продукта каталога с поиском на сервере. */
export function ProductPicker({
	companyId,
	value,
	clearOnSelect = false,
	onChange,
	disabledReason,
	onCreate,
	label,
	ariaLabel,
	placeholder = 'Начните вводить название',
	error,
	unitLabel = (unit) => quantityInput(unit).unit,
}: ProductPickerProps) {
	const [open, setOpen] = useState(false);
	const [input, setInput] = useState('');
	const [q, setQ] = useState('');
	const typed = input.trim();

	useEffect(() => {
		const id = window.setTimeout(() => setQ(typed), searchDelayMs);

		return () => window.clearTimeout(id);
	}, [typed]);

	const products = useQuery({
		...productListQueryOptions(companyId, {
			...(q ? { q } : {}),
			limit: optionLimit,
			offset: 0,
		}),
		enabled: open,
	});
	const items = products.data?.items ?? [];
	const searching = products.isFetching || q !== typed;
	const exact = items.some(
		(product) => product.name.toLowerCase() === typed.toLowerCase()
	);
	const options: Option[] = [
		...(value && !items.some(({ id }) => id === value.id) ? [value] : []),
		...items,
		...(onCreate && typed && !exact && !searching ? [{ create: typed }] : []),
	];

	return (
		<Autocomplete<Option>
			open={open}
			onOpen={() => setOpen(true)}
			onClose={() => setOpen(false)}
			options={options}
			value={value}
			inputValue={input}
			onInputChange={(_, next, reason) => {
				if (!(clearOnSelect && reason === 'reset')) {
					setInput(next);
				}
			}}
			onChange={(_, next) => {
				if (next && isCreate(next)) {
					onCreate?.(next.create);
					return;
				}

				onChange(next);

				if (clearOnSelect) {
					setInput('');
				}
			}}
			filterOptions={(all) => all}
			forcePopupIcon={!clearOnSelect}
			getOptionLabel={(option) =>
				isCreate(option) ? option.create : option.name
			}
			getOptionKey={(option) => (isCreate(option) ? 'create' : option.id)}
			isOptionEqualToValue={(option, selected) =>
				!isCreate(option) && !isCreate(selected) && option.id === selected.id
			}
			getOptionDisabled={(option) =>
				!isCreate(option) && Boolean(disabledReason?.(option))
			}
			loading={searching}
			loadingText='Ищем…'
			noOptionsText={
				products.isError
					? 'Не удалось загрузить продукты'
					: 'Такого продукта нет в каталоге'
			}
			renderOption={({ key, ...props }, option) => {
				if (isCreate(option)) {
					return (
						<Box
							component='li'
							key={key}
							{...props}
							sx={{ '&.MuiAutocomplete-option': { gap: 1 } }}
						>
							<Icon name='add' size='sm' sx={{ color: 'primary.main' }} />
							<Typography sx={{ color: 'primary.main', fontWeight: 600 }}>
								Новый продукт «{option.create}»
							</Typography>
						</Box>
					);
				}

				const reason = disabledReason?.(option);

				return (
					<Box
						component='li'
						key={key}
						{...props}
						sx={{
							'&.MuiAutocomplete-option': {
								justifyContent: 'space-between',
								gap: 1.5,
							},
						}}
					>
						<Box>
							<Typography sx={{ color: 'text.strong' }}>
								{option.name}
							</Typography>
							<Typography variant='caption' sx={{ color: 'text.secondary' }}>
								{reason ?? productNote(option)}
							</Typography>
						</Box>
						<Typography variant='caption' sx={{ color: 'text.secondary' }}>
							{unitLabel(option.unit)}
						</Typography>
					</Box>
				);
			}}
			renderInput={(params) => (
				<TextField
					{...params}
					label={label}
					placeholder={placeholder}
					error={Boolean(error)}
					helperText={error}
					slotProps={{
						...params.slotProps,
						input: clearOnSelect
							? {
									...params.slotProps.input,
									startAdornment: (
										<InputAdornment position='start' sx={{ ml: 0.75 }}>
											<Icon
												name='add'
												size='sm'
												sx={{ color: 'text.secondary' }}
											/>
										</InputAdornment>
									),
								}
							: params.slotProps.input,
						htmlInput: {
							...params.slotProps.htmlInput,
							'aria-label': ariaLabel,
							maxLength: searchTextMaxLength,
						},
					}}
				/>
			)}
		/>
	);
}
