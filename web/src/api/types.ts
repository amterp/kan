export interface Card {
  id: string;
  alias: string;
  alias_explicit: boolean;
  title: string;
  description?: string;
  column: string;
  labels?: string[];
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
}

export interface Column {
  name: string;
  color: string;
}

export interface Label {
  name: string;
  color: string;
  description?: string;
}

export interface BoardConfig {
  id: string;
  name: string;
  columns: Column[];
  default_column: string;
  labels?: Label[];
  custom_fields?: Record<string, CustomFieldSchema>;
}

export interface CustomFieldSchema {
  type: 'string' | 'enum' | 'date';
  values?: string[];
}

export interface CreateCardInput {
  title: string;
  description?: string;
  column?: string;
  labels?: string[];
  parent?: string;
}

export interface UpdateCardInput {
  title?: string;
  description?: string;
  column?: string;
  labels?: string[];
}
