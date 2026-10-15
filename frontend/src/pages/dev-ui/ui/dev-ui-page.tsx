import { Button, Stack, Typography } from '@mui/material';

import { ActionBar } from '@shared/ui/action-bar';
import { PageHeader } from '@shared/ui/page-header';
import { PageContent } from '@shared/ui/page-content';

import {
	ButtonsSection,
	ConnectionSection,
	FeedbackSection,
	FieldsSection,
	ListPartsSection,
	StatusSection,
	TableSection,
} from './components';
import {
	ContrastSection,
	IconsSection,
	PaletteSection,
	RadiusSection,
	TypographySection,
} from './foundations';
import { Section } from './section';

/** Витрина дизайн-системы. Доступна только в режиме разработки. */
export function DevUiPage() {
	return (
		<PageContent
			actionBar={
				<ActionBar
					action={<Button variant='contained'>Подтвердить приёмку</Button>}
				>
					<Typography sx={{ fontWeight: 600, color: 'text.strong' }}>
						7 из 8 позиций решено
					</Typography>
				</ActionBar>
			}
		>
			<PageHeader
				title='Витрина дизайн-системы'
				meta='Токены и базовые компоненты в текущей теме. Переключите тему в навигации, чтобы проверить обе.'
			/>
			<Stack sx={{ gap: 2 }}>
				<PaletteSection />
				<ContrastSection />
				<TypographySection />
				<RadiusSection />
				<ButtonsSection />
				<FieldsSection />
				<StatusSection />
				<TableSection />
				<FeedbackSection />
				<ListPartsSection />
				<ConnectionSection />
				<IconsSection />
				<Section
					title='Панель действий'
					description='Закреплена внизу при прокрутке, в конце списка не перекрывает последний элемент.'
				>
					<Stack sx={{ gap: 1 }}>
						{Array.from({ length: 12 }, (_, i) => (
							<Typography key={i}>Позиция {i + 1}</Typography>
						))}
						<Typography sx={{ color: 'text.strong', fontWeight: 600 }}>
							Последняя позиция списка
						</Typography>
					</Stack>
				</Section>
			</Stack>
		</PageContent>
	);
}
