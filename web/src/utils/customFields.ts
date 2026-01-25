import type { BoardConfig } from '../api/types';

/**
 * Convert custom field values from UI format to API format.
 *
 * The main transformation is tags: the UI uses string[] but the API
 * expects comma-separated strings.
 *
 * @param values - Field values in UI format (tags as string[])
 * @param boardFields - Board's custom field schemas (optional, for type-aware conversion)
 * @returns Field values in API format (tags as comma-separated strings)
 */
export function toApiFieldValues(
  values: Record<string, unknown>,
  boardFields?: BoardConfig['custom_fields']
): Record<string, unknown> {
  const apiFields: Record<string, unknown> = {};

  for (const [fieldName, value] of Object.entries(values)) {
    if (value === undefined) continue;

    // Check if this is a tags field (array → comma-separated string)
    const isTags = boardFields?.[fieldName]?.type === 'tags' || Array.isArray(value);

    if (isTags && Array.isArray(value)) {
      apiFields[fieldName] = (value as string[]).join(',');
    } else {
      apiFields[fieldName] = value;
    }
  }

  return apiFields;
}

/**
 * Convert a single field value to API format.
 * Convenience wrapper for single-field updates.
 */
export function toApiFieldValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return (value as string[]).join(',');
  }
  return value;
}
