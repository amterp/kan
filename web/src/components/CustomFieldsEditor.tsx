import type { BoardConfig, CustomFieldSchema } from '../api/types';
import { useState } from 'react';
import { FIELD_TYPE_ENUM, FIELD_TYPE_ENUM_SET, FIELD_TYPE_FREE_SET, FIELD_TYPE_STRING, FIELD_TYPE_DATE } from '../api/types';
import { stringToColor } from '../utils/badgeColors';
import FieldDescriptionTooltip from './FieldDescriptionTooltip';

interface CustomFieldsEditorProps {
  board: BoardConfig;
  values: Record<string, unknown>;
  onChange: (fieldName: string, value: unknown) => void;
  compact?: boolean; // Tighter spacing for floating panel
}

const MAX_SET_ITEMS = 10;

/**
 * Chip-based input for free-set fields. Enter to add, X to remove,
 * Backspace on empty input to remove the last chip.
 */
function FreeSetField({
  fieldName,
  values,
  onChange,
  wantedIndicator,
  marginClass,
  compact,
  description,
}: {
  fieldName: string;
  values: string[];
  onChange: (fieldName: string, value: unknown) => void;
  wantedIndicator: React.ReactNode;
  marginClass: string;
  compact?: boolean;
  description?: string;
}) {
  const [inputValue, setInputValue] = useState('');
  const [shake, setShake] = useState(false);

  const rejectInput = () => {
    setShake(true);
    setTimeout(() => setShake(false), 400);
    setInputValue('');
  };

  const addValue = (val: string) => {
    const trimmed = val.trim();
    if (!trimmed) {
      setInputValue('');
      return;
    }
    if (values.includes(trimmed) || values.length >= MAX_SET_ITEMS) {
      rejectInput();
      return;
    }
    onChange(fieldName, [...values, trimmed]);
    setInputValue('');
  };

  const removeValue = (val: string) => {
    onChange(fieldName, values.filter((v) => v !== val));
  };

  const atLimit = values.length >= MAX_SET_ITEMS;

  return (
    <div className={marginClass}>
      <label className={`flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 capitalize ${compact ? 'mb-1' : 'mb-2'}`}>
        <span>{fieldName}</span>
        <FieldDescriptionTooltip description={description} />
        {wantedIndicator}
        <span className="text-xs font-normal text-gray-400 dark:text-gray-500 ml-2">{values.length}/{MAX_SET_ITEMS}</span>
      </label>
      {values.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-2">
          {values.map((val) => (
            <span
              key={val}
              className="inline-flex items-center gap-1 px-2 py-0.5 text-xs rounded-full text-white"
              style={{ backgroundColor: stringToColor(val) }}
            >
              {val}
              <button
                type="button"
                onClick={() => removeValue(val)}
                aria-label={`Remove ${val}`}
                className="text-white/80 hover:text-white"
              >
                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </span>
          ))}
        </div>
      )}
      <input
        type="text"
        value={inputValue}
        onChange={(e) => setInputValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            addValue(inputValue);
          } else if (e.key === 'Backspace' && inputValue === '' && values.length > 0) {
            removeValue(values[values.length - 1]);
          }
        }}
        disabled={atLimit}
        className={`w-full border rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors ${
          atLimit
            ? 'border-gray-200 dark:border-gray-700 bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-500 cursor-not-allowed'
            : 'border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white'
        } ${shake ? 'animate-shake border-red-400 dark:border-red-500' : ''}`}
        placeholder={atLimit ? `Limit reached (${MAX_SET_ITEMS})` : `Add ${fieldName}...`}
      />
    </div>
  );
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
    if (current.includes(tagValue)) {
      onChange(fieldName, current.filter((v) => v !== tagValue));
    } else if (current.length < MAX_SET_ITEMS) {
      onChange(fieldName, [...current, tagValue]);
    }
  };

  const renderField = (fieldName: string, schema: CustomFieldSchema) => {
    const currentValue = values[fieldName];
    const marginClass = compact ? 'mb-2' : 'mb-4';

    // Render wanted indicator
    const wantedIndicator = schema.wanted ? (
      <span
        className="text-amber-500 ml-1"
        title="This field is wanted by the board configuration"
      >
        *
      </span>
    ) : null;

    switch (schema.type) {
      case FIELD_TYPE_ENUM:
        return (
          <div className={marginClass} key={fieldName}>
            <label className="flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">
              <span>{fieldName}</span>
              <FieldDescriptionTooltip description={schema.description} />
              {wantedIndicator}
            </label>
            <select
              value={(currentValue as string) || ''}
              onChange={(e) => onChange(fieldName, e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white dark:bg-gray-700 dark:text-white"
            >
              <option value="">None</option>
              {schema.options?.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.description ? `${opt.value} - ${opt.description}` : opt.value}
                </option>
              ))}
            </select>
          </div>
        );

      case FIELD_TYPE_ENUM_SET: {
        const selectedValues = Array.isArray(currentValue) ? (currentValue as string[]) : [];
        return (
          <div className={marginClass} key={fieldName}>
            <label className={`flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 capitalize ${compact ? 'mb-1' : 'mb-2'}`}>
              <span>{fieldName}</span>
              <FieldDescriptionTooltip description={schema.description} />
              {wantedIndicator}
            </label>
            <div className="flex flex-wrap gap-2">
              {schema.options?.map((opt) => {
                const isSelected = selectedValues.includes(opt.value);
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => toggleTagValue(fieldName, opt.value)}
                    title={opt.description || undefined}
                    className={`px-2 py-0.5 text-xs rounded-full transition-all ${
                      isSelected
                        ? 'text-white ring-2 ring-offset-1'
                        : 'text-white opacity-60 hover:opacity-80'
                    }`}
                    style={{
                      backgroundColor: opt.color || stringToColor(opt.value),
                      boxShadow: isSelected
                        ? `0 0 0 2px white, 0 0 0 4px ${opt.color || stringToColor(opt.value)}`
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

      case FIELD_TYPE_FREE_SET: {
        const selectedValues = Array.isArray(currentValue) ? (currentValue as string[]) : [];
        return (
          <FreeSetField
            key={fieldName}
            fieldName={fieldName}
            values={selectedValues}
            onChange={onChange}
            wantedIndicator={wantedIndicator}
            marginClass={marginClass}
            compact={compact}
            description={schema.description}
          />
        );
      }

      case FIELD_TYPE_STRING:
        return (
          <div className={marginClass} key={fieldName}>
            <label className="flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">
              <span>{fieldName}</span>
              <FieldDescriptionTooltip description={schema.description} />
              {wantedIndicator}
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
            <label className="flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">
              <span>{fieldName}</span>
              <FieldDescriptionTooltip description={schema.description} />
              {wantedIndicator}
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
