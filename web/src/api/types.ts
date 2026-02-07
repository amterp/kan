// Missing wanted field option returned by API
export interface MissingWantedFieldOption {
  value: string;
  description?: string;
}

// Missing wanted field info returned by API
export interface MissingWantedField {
  name: string;
  type: string;
  description?: string;
  options?: MissingWantedFieldOption[];
}

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
  missing_wanted_fields?: MissingWantedField[];
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
  description?: string;
  card_ids?: string[];
}

export interface CustomFieldOption {
  value: string;
  color?: string;
  description?: string;
}

// Custom field type constants - keep in sync with internal/model/board.go
export const FIELD_TYPE_STRING = 'string' as const;
export const FIELD_TYPE_ENUM = 'enum' as const;
export const FIELD_TYPE_ENUM_SET = 'enum-set' as const;
export const FIELD_TYPE_FREE_SET = 'free-set' as const;
export const FIELD_TYPE_DATE = 'date' as const;

export const VALID_FIELD_TYPES = [
  FIELD_TYPE_STRING,
  FIELD_TYPE_ENUM,
  FIELD_TYPE_ENUM_SET,
  FIELD_TYPE_FREE_SET,
  FIELD_TYPE_DATE,
] as const;

export type FieldType = (typeof VALID_FIELD_TYPES)[number];

export interface CustomFieldSchema {
  type: FieldType;
  options?: CustomFieldOption[];
  wanted?: boolean;
  description?: string;
}

export interface CardDisplayConfig {
  type_indicator?: string;
  badges?: string[];
  metadata?: string[];
}

export interface LinkRule {
  name: string;
  pattern: string;
  url: string;
}

export interface BoardConfig {
  id: string;
  name: string;
  columns: Column[];
  default_column: string;
  custom_fields?: Record<string, CustomFieldSchema>;
  card_display?: CardDisplayConfig;
  link_rules?: LinkRule[];
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

// Hook execution result from pattern hooks
export interface HookInfo {
  name: string;
  success: boolean;
  output?: string;
  error?: string;
}

// Response from creating a card (includes hook results)
export interface CreateCardResponse {
  card: Card;
  hook_results?: HookInfo[];
  missing_wanted_fields?: MissingWantedField[];
}

export interface CreateColumnInput {
  name: string;
  color?: string;
  description?: string;
  position?: number;
}

export interface UpdateColumnInput {
  name?: string;
  color?: string;
  description?: string;
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

// Cross-project types

export interface BoardEntry {
  project_name: string;
  project_path: string;
  board_name: string;
}

export interface SkippedProject {
  name: string;
  path: string;
  reason: string;
}

export interface AllBoardsResponse {
  boards: BoardEntry[];
  current_project_path: string;
  skipped?: SkippedProject[];
}

export interface SwitchResponse {
  project_name: string;
  boards: string[];
}
