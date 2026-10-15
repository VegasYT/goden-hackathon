import { TextField } from '@mui/material';
import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';

import { createCompany, createPoint } from '@entities/organization';
import {
	companyWithoutPoints,
	meQueryOptions,
	refreshProfile,
} from '@entities/profile';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { FormDialog } from '@shared/ui/form-dialog';
import { InfoNote } from '@shared/ui/info-note';

interface NewCompany {
	companyName: string;
	pointName: string;
}

/** Создаёт компанию, если своей компании без точек нет, затем её первую точку; профиль перечитывается после каждого шага. */
function useCreateCompanyWithPoint() {
	const queryClient = useQueryClient();
	const companyKey = useIdempotencyKey();
	const pointKey = useIdempotencyKey();

	return useMutation({
		mutationFn: async ({ companyName, pointName }: NewCompany) => {
			let company = companyWithoutPoints(
				await queryClient.ensureQueryData(meQueryOptions)
			);

			if (!company) {
				await createCompany(companyName, companyKey({ name: companyName }));
				company = companyWithoutPoints(await refreshProfile(queryClient));
			}

			if (!company) {
				throw new Error(
					'Компания создана, но не нашлась в профиле. Обновите страницу'
				);
			}

			const point = await createPoint(
				company.id,
				pointName,
				pointKey({ company: company.id, name: pointName })
			);

			await refreshProfile(queryClient);
			return point;
		},
	});
}

/** Новая компания с первой точкой; если своя компания без точек уже есть, создаёт только её точку. */
export function NewCompanyDialog({ onClose }: { onClose: () => void }) {
	const { data: profile } = useSuspenseQuery(meQueryOptions);
	const navigate = useNavigate();
	const mutation = useCreateCompanyWithPoint();
	const company = companyWithoutPoints(profile);
	const [companyName, setCompanyName] = useState('');
	const [pointName, setPointName] = useState('');
	const [tried, setTried] = useState(false);
	const companyError =
		tried && !company && !companyName.trim()
			? 'Введите название компании'
			: undefined;
	const pointError =
		tried && !pointName.trim() ? 'Введите название точки' : undefined;

	const submit = () => {
		setTried(true);

		const next = {
			companyName: companyName.trim(),
			pointName: pointName.trim(),
		};

		if ((!company && !next.companyName) || !next.pointName) {
			return;
		}

		mutation.mutate(next, {
			onSuccess: (point) => {
				onClose();
				void navigate({
					to: '/points/$pointId',
					params: { pointId: point.id },
				});
			},
		});
	};

	return (
		<FormDialog
			title={
				company ? `Первая точка компании «${company.name}»` : 'Новая компания'
			}
			submitLabel={company ? 'Создать точку' : 'Создать компанию'}
			pending={mutation.isPending}
			error={mutation.error?.message}
			onSubmit={submit}
			onClose={onClose}
		>
			{!company && (
				<TextField
					label='Название компании'
					autoFocus
					value={companyName}
					onChange={(event) => setCompanyName(event.target.value)}
					error={!!companyError}
					helperText={companyError ?? 'Название видят сотрудники всех точек'}
				/>
			)}
			<TextField
				label='Первая точка'
				placeholder='Например, Центр'
				autoFocus={!!company}
				value={pointName}
				onChange={(event) => setPointName(event.target.value)}
				error={!!pointError}
				helperText={pointError}
			/>
			<InfoNote>
				Каталог продуктов и блюд у новой компании свой и пока пустой.
			</InfoNote>
		</FormDialog>
	);
}
