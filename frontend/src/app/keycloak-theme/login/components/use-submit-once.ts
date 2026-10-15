import { useRef, useState, type FormEvent } from 'react';

/** Пропускает только первую отправку формы и блокирует основное действие до ответа Keycloak. */
export function useSubmitOnce() {
	const submitted = useRef(false);
	const [isSubmitting, setIsSubmitting] = useState(false);

	const onSubmit = (event: FormEvent<HTMLFormElement>) => {
		// Второе нажатие может прийти раньше, чем кнопка станет неактивной.
		if (submitted.current) {
			event.preventDefault();

			return;
		}

		submitted.current = true;
		setIsSubmitting(true);
	};

	return { isSubmitting, onSubmit };
}
