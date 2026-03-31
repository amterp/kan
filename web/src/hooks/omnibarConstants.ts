// Slash-command prefix that triggers board-switching mode in the omnibar.
// Shared between useOmnibar (mode detection) and useBoardSwitcher (query parsing).
export const BOARD_PREFIX = '/board ';

// Slash-command that toggles compact view mode.
export const COMPACT_COMMAND = '/compact';

// Slash-command that toggles slim view mode (vertical column layout).
export const SLIM_COMMAND = '/slim';

// Structured command registry for autocomplete.
export interface SlashCommand {
  /** The command string the user types, e.g. "/board" */
  command: string;
  /** Brief description shown in the autocomplete dropdown */
  description: string;
  /**
   * If true, selecting this command inserts it into the input (with trailing space)
   * rather than executing immediately. Used for prefix commands like /board.
   */
  insertsIntoInput: boolean;
}

export const SLASH_COMMANDS: SlashCommand[] = [
  { command: '/board', description: 'Switch to another board', insertsIntoInput: true },
  { command: '/compact', description: 'Toggle compact view', insertsIntoInput: false },
  { command: '/slim', description: 'Toggle slim view (vertical columns)', insertsIntoInput: false },
];
