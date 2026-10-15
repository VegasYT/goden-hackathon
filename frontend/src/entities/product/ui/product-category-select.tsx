import { MenuItem, TextField } from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import { productCategoriesQueryOptions } from '../api/product-queries';

/** Фильтр по категории каталога компании с пунктом «Все категории»; пустое значение — все. */
export function ProductCategorySelect({
	companyId,
	value,
	onChange,
	fullWidth = false,
}: {
	companyId: string;
	value: string;
	onChange: (category: string) => void;
	/** В панели фильтров: на всю ширину и с видимой подписью. */
	fullWidth?: boolean;
}) {
	const categories = useQuery(productCategoriesQueryOptions(companyId));
	const names = (categories.data ?? []).map(({ name }) => name);

	return (
		<TextField
			select
			// В строке фильтров подпись не видна: её заменяет «Все категории».
			label={fullWidth ? 'Категория' : undefined}
			value={value}
			onChange={(event) => onChange(event.target.value)}
			fullWidth={fullWidth}
			sx={fullWidth ? undefined : { width: 190 }}
			slotProps={{
				select: { displayEmpty: true },
				htmlInput: { 'aria-label': 'Категория' },
			}}
		>
			<MenuItem value=''>Все категории</MenuItem>
			{/* Категория из адреса может не совпасть с каталогом: её всё равно показываем выбранной. */}
			{[...new Set([...names, value].filter(Boolean))].map((category) => (
				<MenuItem key={category} value={category}>
					{category}
				</MenuItem>
			))}
		</TextField>
	);
}
