import {
	Box,
	Chip,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import { ProductHistoryLink } from '@entities/product';
import { reasonLabel } from '@entities/delivery';
import { formatAmount } from '@shared/lib/quantity';
import { EmptyState } from '@shared/ui/empty-state';
import {
	SegmentedControl,
	type SegmentedOption,
} from '@shared/ui/segmented-control';

import {
	writeOffsQueryOptions,
	type PeriodParams,
	type WriteOffStatistic,
} from '../api/analytics';
import { formatCount } from '../model/labels';
import type { AnalyticsSearch } from '../model/search';
import { writeOffsByProduct, writeOffsByReason } from '../model/write-offs';
import { Section } from './section';

type View = AnalyticsSearch['writeOffs'];

const viewOptions: SegmentedOption<View>[] = [
	{ value: 'products', label: 'По продуктам' },
	{ value: 'reasons', label: 'По причинам' },
];

/** Сколько продуктов причины назвать, остальные - числом. */
const reasonProductsShown = 3;

interface WriteOffsSectionProps {
	pointId: string;
	period: PeriodParams;
	view: View;
	onViewChange: (view: View) => void;
	wide: boolean;
	mayViewStock: boolean;
}

/** Число операций по причинам: в отличие от количеств, их можно сложить. */
function ReasonTotals({ items }: { items: WriteOffStatistic[] }) {
	return (
		<Stack
			direction='row'
			sx={{
				flexWrap: 'wrap',
				columnGap: 2,
				rowGap: 0.5,
				px: 2,
				py: 1.25,
				borderBottom: '1px solid',
				borderColor: 'divider',
			}}
		>
			{writeOffsByReason(items).map((row) => (
				<Typography
					key={row.reason}
					variant='body2'
					sx={{ color: 'text.secondary' }}
				>
					{reasonLabel(row.reason)}{' '}
					<Box component='b' sx={{ color: 'text.strong', fontWeight: 600 }}>
						{formatCount(row.operations)}
					</Box>
				</Typography>
			))}
			<Typography variant='body2' sx={{ color: 'text.tertiary' }}>
				операций
			</Typography>
		</Stack>
	);
}

/** Списания за период по продуктам или по причинам. */
export function WriteOffsSection({
	pointId,
	period,
	view,
	onViewChange,
	wide,
	mayViewStock,
}: WriteOffsSectionProps) {
	const writeOffs = useQuery(writeOffsQueryOptions(pointId, period));
	const name = (product: WriteOffStatistic['product']) => (
		<ProductHistoryLink
			pointId={pointId}
			product={product}
			linked={mayViewStock}
		/>
	);

	return (
		<Section
			title='Списания'
			description='Продукты с наибольшим списанием в своей единице'
			subject='списания'
			headingLevel='h3'
			query={writeOffs}
			action={
				<SegmentedControl
					label='Группировка списаний'
					options={viewOptions}
					value={view}
					onChange={onViewChange}
					fullWidth={!wide}
				/>
			}
			empty={
				<EmptyState icon='writtenOff' title='За период ничего не списано' />
			}
			footer='Килограммы, литры и штуки не складываются, поэтому итог по причинам - в операциях.'
		>
			{(items) =>
				view === 'products' ? (
					<>
						<ReasonTotals items={items} />
						<Table aria-label='Списания по продуктам'>
							<TableHead>
								<TableRow>
									<TableCell sx={{ pl: 2 }}>Продукт</TableCell>
									<TableCell>Причины</TableCell>
									<TableCell align='right'>Списано</TableCell>
									<TableCell align='right' sx={{ pr: 2 }}>
										Операций
									</TableCell>
								</TableRow>
							</TableHead>
							<TableBody>
								{writeOffsByProduct(items).map((row) => (
									<TableRow key={row.product.id}>
										<TableCell sx={{ pl: 2 }}>{name(row.product)}</TableCell>
										<TableCell>
											<Stack
												direction='row'
												sx={{ gap: 0.5, flexWrap: 'wrap' }}
											>
												{row.reasons.map((reason) => (
													<Chip key={reason} label={reasonLabel(reason)} />
												))}
											</Stack>
										</TableCell>
										<TableCell
											align='right'
											sx={{
												fontWeight: 500,
												color: 'text.strong',
												whiteSpace: 'nowrap',
											}}
										>
											{formatAmount(row.quantity, row.product.unit)}
										</TableCell>
										<TableCell align='right' sx={{ pr: 2 }}>
											{formatCount(row.operations)}
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					</>
				) : (
					<Table aria-label='Списания по причинам'>
						<TableHead>
							<TableRow>
								<TableCell sx={{ pl: 2 }}>Причина</TableCell>
								<TableCell align='right'>Операций</TableCell>
								<TableCell sx={{ pr: 2 }}>Продукты</TableCell>
							</TableRow>
						</TableHead>
						<TableBody>
							{writeOffsByReason(items).map((row) => {
								const rest = row.products.length - reasonProductsShown;

								return (
									<TableRow key={row.reason}>
										<TableCell
											sx={{ pl: 2, fontWeight: 500, color: 'text.strong' }}
										>
											{reasonLabel(row.reason)}
										</TableCell>
										<TableCell align='right'>
											{formatCount(row.operations)}
										</TableCell>
										<TableCell sx={{ pr: 2 }}>
											{row.products
												.slice(0, reasonProductsShown)
												.map(({ product, quantity }, index) => (
													<Box component='span' key={product.id}>
														{index > 0 && ', '}
														{name(product)}{' '}
														{formatAmount(quantity, product.unit)}
													</Box>
												))}
											{rest > 0 && ` и ещё ${rest}`}
										</TableCell>
									</TableRow>
								);
							})}
						</TableBody>
					</Table>
				)
			}
		</Section>
	);
}
