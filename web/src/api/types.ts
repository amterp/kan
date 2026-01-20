export interface Card {
  id: string;
  alias: string;
  alias_explicit: boolean;
  title: string;
  description?: string;
  column: string;
  parent?: string;
  creator: string;
  created_at_millis: number;
  updated_at_millis: number;
  comments?: Comment[];
  [key: string]: unknown; // custom fields
}

export interface Comment {
  id: string;
  body: string;
  author: string;
  created_at_millis: number;
  updated_at_millis?: number;
}

export interface Column {
  name: string;
  color: string;
  card_ids?: string[];
}

export interface CustomFieldOption {
  value: string;
  color?: string;
}

// Custom field type constants - keep in sync with internal/model/board.go
export const FIELD_TYPE_STRING = 'string' as const;
export const FIELD_TYPE_ENUM = 'enum' as const;
export const FIELD_TYPE_TAGS = 'tags' as const;
export const FIELD_TYPE_DATE = 'date' as const;

export const VALID_FIELD_TYPES = [
  FIELD_TYPE_STRING,
  FIELD_TYPE_ENUM,
  FIELD_TYPE_TAGS,
  FIELD_TYPE_DATE,
] as const;

export type FieldType = (typeof VALID_FIELD_TYPES)[number];

export interface CustomFieldSchema {
  type: FieldType;
  options?: CustomFieldOption[];
}

export interface CardDisplayConfig {
  type_indicator?: string;
  badges?: string[];
  metadata?: string[];
}

export interface BoardConfig {
  id: string;
  name: string;
  columns: Column[];
  default_column: string;
  custom_fields?: Record<string, CustomFieldSchema>;
  card_display?: CardDisplayConfig;
}

export interface CreateCardInput {
  title: string;
  description?: string;
  column?: string;
  parent?: string;
  custom_fields?: Record<string, unknown>;
}

export interface UpdateCardInput {
  title?: string;
  description?: string;
  column?: string;
  custom_fields?: Record<string, unknown>;
}

export interface CreateColumnInput {
  name: string;
  color?: string;
  position?: number;
}

export interface UpdateColumnInput {
  name?: string;
  color?: string;
}

export interface FaviconConfig {
  background: string;
  icon_type: string;
  letter: string;
  emoji: string;
}

export interface ProjectConfig {
  name: string;
  favicon: FaviconConfig;
}
