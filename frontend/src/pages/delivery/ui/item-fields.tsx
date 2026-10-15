import { Stack } from '@mui/material';

import {
	allocationText,
	AllocationWarning,
	AutoExpiredReason,
	itemName,
	itemUnit,
	WriteOffCommentField,
	WriteOffReasonSelect,
	type DeliveryItem,
} from '@entities/delivery';
import { formatQuantityValue } from '@shared/lib/quantity';
import { UnitQuantityField } from '@shared/ui/quantity-field';

import {
	allocated,
	type Decision,
	type DecisionProblem,
	type Part,
} from '../model/decision';
import type { ApprovalForm } from '../model/form';
import { partLabels } from './parts';

interface ItemProps {
	form: ApprovalForm;
	index: number;
	item: DeliveryItem;
	decision: Decision;
}

/** Поле количества одной части решения в единице показа; в таблице подпись даёт заголовок колонки. */
export function QuantityInput({
	form,
	index,
	item,
	decision,
	part,
	hideLabel = false,
}: ItemProps & { part: Part; hideLabel?: boolean }) {
	const unit = itemUnit(item);
	const name = itemName(item);
	const locked = part === 'accepted' && decision.expired;

	return (
		<form.Field name={`items[${index}].${part}`}>
			{(field) => (
				<UnitQuantityField
					label={partLabels[part]}
					accessibleLabel={`${name}: ${partLabels[part].toLowerCase()}`}
					hideLabel={hideLabel}
					fullWidth
					unit={unit}
					max={item.declared_quantity}
					value={field.state.value}
					onValueChange={(value) => field.handleChange(value)}
					disabled={locked}
					helperText={
						locked && decision.previousAccepted
							? `было ${formatQuantityValue(decision.previousAccepted, unit)}`
							: undefined
					}
				/>
			)}
		</form.Field>
	);
}

function ReasonSelect({
	form,
	index,
	field: fieldName,
	label,
	missing,
}: {
	form: ApprovalForm;
	index: number;
	field: 'rejectReason' | 'quarantineReason';
	label: string;
	missing: boolean;
}) {
	return (
		<form.Field name={`items[${index}].${fieldName}`}>
			{(field) => (
				<WriteOffReasonSelect
					label={label}
					value={field.state.value}
					onChange={(reason) => field.handleChange(reason)}
					onBlur={field.handleBlur}
					missing={missing}
					error={missing && field.state.meta.isBlurred}
				/>
			)}
		</form.Field>
	);
}

/** Сообщение о распределении количества позиции, если оно не сходится. */
function allocationMessage(
	item: DeliveryItem,
	decision: Decision,
	problems: DecisionProblem[]
): string | undefined {
	if (problems.includes('accepted-expired')) {
		return 'Срок истёк — принять нельзя';
	}

	return allocationText(
		item.declared_quantity,
		allocated(decision),
		itemUnit(item),
		'declared'
	);
}

/** Причины и комментарий: появляются, когда есть отклонённое или карантинное количество. */
export function ReasonFields({
	form,
	index,
	item,
	decision,
	problems,
}: ItemProps & { problems: DecisionProblem[] }) {
	const rejected = (decision.rejected ?? 0) > 0;
	const quarantined = (decision.quarantined ?? 0) > 0;
	const message = allocationMessage(item, decision, problems);

	return (
		<Stack sx={{ gap: 1, minWidth: 0 }}>
			{message && <AllocationWarning>{message}</AllocationWarning>}
			{rejected &&
				(decision.autoReason ? (
					<AutoExpiredReason />
				) : (
					<ReasonSelect
						form={form}
						index={index}
						field='rejectReason'
						label='Причина отклонения'
						missing={problems.includes('reject-reason')}
					/>
				))}
			{quarantined && (
				<ReasonSelect
					form={form}
					index={index}
					field='quarantineReason'
					label='Причина карантина'
					missing={problems.includes('quarantine-reason')}
				/>
			)}
			{(rejected || quarantined) && (
				<form.Field name={`items[${index}].comment`}>
					{(field) => (
						<WriteOffCommentField
							value={field.state.value}
							onChange={(value) => field.handleChange(value)}
							onBlur={field.handleBlur}
							missing={problems.includes('comment')}
							error={problems.includes('comment') && field.state.meta.isBlurred}
						/>
					)}
				</form.Field>
			)}
		</Stack>
	);
}
