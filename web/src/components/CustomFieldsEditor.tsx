import type { BoardConfig, CustomFieldSchema } from '../api/types';
import { FIELD_TYPE_ENUM, FIELD_TYPE_TAGS, FIELD_TYPE_STRING, FIELD_TYPE_DATE } from '../api/types';

interface CustomFieldsEditorProps {
  board: BoardConfig;
  values: Record<string, unknown>;
  onChange: (fieldName: string, value: unknown) => void;
  compact?: boolean; // Tighter spacing for floating panel
}

/**
 * Reusable editor for custom fields defined on a board.
 * Used in CardEditModal and FloatingFieldPanel.
 */
export default function CustomFieldsEditor({
  board,
  values,
  onChange,
  compact = false,
}: CustomFieldsEditorProps) {
  if (!board.custom_fields || Object.keys(board.custom_fields).length === 0) {
    return null;
  }

  const toggleTagValue = (fieldName: string, tagValue: string) => {
    const current = Array.isArray(values[fieldName]) ? (values[fieldName] as string[]) : [];
    const newValues = current.includes(tagValue)
      ? current.filter((v) => v !== tagValue)
      : [...current, tagValue];
    onChange(fieldName, newValues);
  };

  const renderField = (fieldName: string, schema: CustomFieldSchema) => {
    const currentValue = values[fieldName];
    const marginClass = compact ? 'mb-2' : 'mb-4';

    switch (schema.type) {
      case FIELD_TYPE_ENUM:
        return (
          <div className={marginClass} key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">
              {fieldName}
            </label>
            <select
              value={(currentValue as string) || ''}
              onChange={(e) => onChange(fieldName, e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white dark:bg-gray-700 dark:text-white"
            >
              <option value="">None</option>
              {schema.options?.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.value}
                </option>
              ))}
            </select>
          </div>
        );

      case FIELD_TYPE_TAGS: {
        const selectedTags = Array.isArray(currentValue) ? (currentValue as string[]) : [];
        return (
          <div className={marginClass} key={fieldName}>
            <label className={`block text-sm font-medium text-gray-700 dark:text-gray-300 capitalize ${compact ? 'mb-1' : 'mb-2'}`}>
              {fieldName}
            </label>
            <div className="flex flex-wrap gap-2">
              {schema.options?.map((opt) => {
                const isSelected = selectedTags.includes(opt.value);
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => toggleTagValue(fieldName, opt.value)}
                    className={`px-2 py-0.5 text-xs rounded-full transition-all ${
                      isSelected
                        ? 'text-white ring-2 ring-offset-1'
                        : 'text-white opacity-50 hover:opacity-75'
                    }`}
                    style={{
                      backgroundColor: opt.color || '#6b7280',
                      boxShadow: isSelected
                        ? `0 0 0 2px white, 0 0 0 4px ${opt.color || '#6b7280'}`
                        : undefined,
                    }}
                  >
                    {opt.value}
                  </button>
                );
              })}
            </div>
          </div>
        );
      }

      case FIELD_TYPE_STRING:
        return (
          <div className={marginClass} key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">
              {fieldName}
            </label>
            <input
              type="text"
              value={(currentValue as string) || ''}
              onChange={(e) => onChange(fieldName, e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder={`Enter ${fieldName}...`}
            />
          </div>
        );

      case FIELD_TYPE_DATE:
        return (
          <div className={marginClass} key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">
              {fieldName}
            </label>
            <input
              type="date"
              value={(currentValue as string) || ''}
              onChange={(e) => onChange(fieldName, e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <>
      {Object.entries(board.custom_fields).map(([fieldName, schema]) =>
        renderField(fieldName, schema)
      )}
    </>
  );
}
