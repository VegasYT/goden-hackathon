import { Tabs } from '@mui/material';
import type { ReactNode } from 'react';

import { useWideLayout } from '@shared/ui/theme';

/** Вкладки раздела под шапкой страницы: `TabLink` на дочерние роуты, на планшете во всю ширину. */
export function SectionTabs({
	value,
	label,
	children,
}: {
	value: string;
	/** Название набора вкладок для программ чтения с экрана: «Разделы склада». */
	label: string;
	children: ReactNode;
}) {
	const wide = useWideLayout();

	return (
		<Tabs
			value={value}
			aria-label={label}
			variant={wide ? 'standard' : 'fullWidth'}
			sx={{ mb: 2.5, borderBottom: 1, borderColor: 'divider' }}
		>
			{children}
		</Tabs>
	);
}
