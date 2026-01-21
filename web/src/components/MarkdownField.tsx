import { useState, useRef, useEffect, useCallback } from 'react';
import MarkdownView from './MarkdownView';

// Storage key for monospace preference
const MONOSPACE_STORAGE_KEY = 'kan-markdown-monospace';

// Platform detection for keyboard shortcut display
const isMac = typeof navigator !== 'undefined' && navigator.platform.toUpperCase().indexOf('MAC') >= 0;
const modKey = isMac ? 'Cmd' : 'Ctrl';

// Formatting utilities
// Uses execCommand to integrate with browser's native undo stack
function applyFormatting(
  textareaRef: React.RefObject<HTMLTextAreaElement | null>,
  value: string,
  onChange: (v: string) => void,
  prefix: string,
  suffix: string = prefix
) {
  const textarea = textareaRef.current;
  if (!textarea) return;

  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const selectedText = value.substring(start, end);
  const newText = prefix + selectedText + suffix;

  // Focus and select the range we want to replace
  textarea.focus();
  textarea.setSelectionRange(start, end);

  // Use execCommand to insert text - this integrates with native undo
  // The input event will trigger onChange to update React state
  const success = document.execCommand('insertText', false, newText);

  if (!success) {
    // Fallback for browsers that don't support execCommand
    const before = value.substring(0, start);
    const after = value.substring(end);
    onChange(before + newText + after);
  }

  // Position cursor after the inserted text (after prefix, selecting original text area)
  requestAnimationFrame(() => {
    textarea.setSelectionRange(start + prefix.length, end + prefix.length);
  });
}

function applyLinkFormatting(
  textareaRef: React.RefObject<HTMLTextAreaElement | null>,
  value: string,
  onChange: (v: string) => void
) {
  const textarea = textareaRef.current;
  if (!textarea) return;

  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const selectedText = value.substring(start, end);

  // Focus and select the range
  textarea.focus();
  textarea.setSelectionRange(start, end);

  if (!selectedText) {
    // No selection: insert placeholder
    const newText = '[link text](url)';
    const success = document.execCommand('insertText', false, newText);
    if (!success) {
      const before = value.substring(0, start);
      const after = value.substring(end);
      onChange(before + newText + after);
    }
    requestAnimationFrame(() => {
      textarea.setSelectionRange(start + 1, start + 10); // Select "link text"
    });
    return;
  }

  // Wrap selection as link
  const newText = '[' + selectedText + '](url)';
  const success = document.execCommand('insertText', false, newText);
  if (!success) {
    const before = value.substring(0, start);
    const after = value.substring(end);
    onChange(before + newText + after);
  }
  requestAnimationFrame(() => {
    // Select "url" for easy replacement
    textarea.setSelectionRange(start + selectedText.length + 3, start + selectedText.length + 6);
  });
}

// Monospace preference hook
function useMonospacePreference(): [boolean, () => void] {
  const [isMonospace, setIsMonospace] = useState(() => {
    try {
      if (typeof window === 'undefined') return false;
      return localStorage.getItem(MONOSPACE_STORAGE_KEY) === 'true';
    } catch {
      return false;
    }
  });

  const toggleMonospace = useCallback(() => {
    setIsMonospace((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(MONOSPACE_STORAGE_KEY, String(next));
      } catch {
        // Ignore localStorage errors
      }
      return next;
    });
  }, []);

  return [isMonospace, toggleMonospace];
}

// Icon components
function BoldIcon({ className = 'w-4 h-4' }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 4h8a4 4 0 0 1 0 8H6V4z" />
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 12h9a4 4 0 0 1 0 8H6v-8z" />
    </svg>
  );
}

function ItalicIcon({ className = 'w-4 h-4' }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
      <line x1="19" y1="4" x2="10" y2="4" />
      <line x1="14" y1="20" x2="5" y2="20" />
      <line x1="15" y1="4" x2="9" y2="20" />
    </svg>
  );
}

function LinkIcon({ className = 'w-4 h-4' }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13.828 10.172a4 4 0 0 0-5.656 0l-4 4a4 4 0 1 0 5.656 5.656l1.102-1.101m-.758-4.899a4 4 0 0 1 5.656 0l4-4a4 4 0 0 0-5.656-5.656l-1.1 1.1" />
    </svg>
  );
}

function MonospaceIcon({ className = 'w-4 h-4' }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2z" />
    </svg>
  );
}

// Toolbar button component
interface ToolbarButtonProps {
  icon: React.ReactNode;
  label: string;
  shortcut?: string;
  onClick: () => void;
  isActive?: boolean;
}

function ToolbarButton({ icon, label, shortcut, onClick, isActive }: ToolbarButtonProps) {
  const title = shortcut ? `${label} (${shortcut})` : label;

  return (
    <button
      type="button"
      onClick={onClick}
      onMouseDown={(e) => e.preventDefault()} // Prevent stealing focus from textarea
      title={title}
      className={`p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors ${
        isActive
          ? 'bg-gray-200 dark:bg-gray-600 text-blue-600 dark:text-blue-400'
          : 'text-gray-600 dark:text-gray-400'
      }`}
    >
      {icon}
    </button>
  );
}

interface MarkdownFieldProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  minHeight?: string;
  alwaysEditing?: boolean; // When true, never toggles to preview mode
  onSubmit?: () => void; // Called on Cmd+Enter when alwaysEditing is true
  autoFocus?: boolean; // Focus on initial mount when starting in edit mode
}

export default function MarkdownField({
  value,
  onChange,
  placeholder = '',
  minHeight = 'min-h-48',
  alwaysEditing = false,
  onSubmit,
  autoFocus = false,
}: MarkdownFieldProps) {
  const [isEditing, setIsEditing] = useState(alwaysEditing || !value?.trim());
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [isMonospace, toggleMonospace] = useMonospacePreference();

  // Track previous isEditing state to detect transitions
  const prevIsEditingRef = useRef<boolean | null>(null);

  // Toolbar button handlers
  const handleBold = () => applyFormatting(textareaRef, value, onChange, '**');
  const handleItalic = () => applyFormatting(textareaRef, value, onChange, '*');
  const handleLink = () => applyLinkFormatting(textareaRef, value, onChange);

  useEffect(() => {
    if (isEditing && textareaRef.current) {
      const wasEditing = prevIsEditingRef.current;
      // Focus if:
      // 1. autoFocus is true and this is the initial mount (wasEditing is null), OR
      // 2. User clicked to enter edit mode (wasEditing was false)
      const shouldFocus = (autoFocus && wasEditing === null) || wasEditing === false;

      if (shouldFocus) {
        textareaRef.current.focus();
        const len = textareaRef.current.value.length;
        textareaRef.current.setSelectionRange(len, len);
      }
    }
    prevIsEditingRef.current = isEditing;
  }, [isEditing, autoFocus]);

  const handleBlur = () => {
    if (!alwaysEditing && value?.trim()) {
      setIsEditing(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Escape' && !alwaysEditing && value?.trim()) {
      e.preventDefault();
      setIsEditing(false);
      return;
    }

    // Cmd+Enter exits edit mode (stops propagation so modal doesn't save)
    // In alwaysEditing mode, call onSubmit if provided
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      if (value?.trim()) {
        e.preventDefault();
        e.nativeEvent.stopImmediatePropagation();
        if (alwaysEditing && onSubmit) {
          onSubmit();
        } else if (!alwaysEditing) {
          setIsEditing(false);
        }
      }
      return;
    }

    // Formatting shortcuts
    if (e.metaKey || e.ctrlKey) {
      switch (e.key.toLowerCase()) {
        case 'b':
          e.preventDefault();
          handleBold();
          break;
        case 'i':
          e.preventDefault();
          handleItalic();
          break;
        case 'k':
          e.preventDefault();
          handleLink();
          break;
      }
    }
  };

  if (isEditing) {
    return (
      <div className="flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center gap-1 p-1 bg-gray-50 dark:bg-gray-800 rounded-t-md border border-b-0 border-gray-300 dark:border-gray-600">
          <ToolbarButton
            icon={<BoldIcon />}
            label="Bold"
            shortcut={`${modKey}+B`}
            onClick={handleBold}
          />
          <ToolbarButton
            icon={<ItalicIcon />}
            label="Italic"
            shortcut={`${modKey}+I`}
            onClick={handleItalic}
          />
          <ToolbarButton
            icon={<LinkIcon />}
            label="Link"
            shortcut={`${modKey}+K`}
            onClick={handleLink}
          />
          {/* Separator */}
          <div className="w-px h-4 bg-gray-300 dark:bg-gray-600 mx-1" />
          <ToolbarButton
            icon={<MonospaceIcon />}
            label={isMonospace ? 'Proportional font' : 'Monospace font'}
            onClick={toggleMonospace}
            isActive={isMonospace}
          />
        </div>
        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onBlur={handleBlur}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className={`w-full ${minHeight} border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-t-none rounded-b-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none ${isMonospace ? 'font-mono' : ''}`}
        />
      </div>
    );
  }

  return (
    <div
      onClick={() => setIsEditing(true)}
      className={`${minHeight} overflow-y-auto border border-transparent hover:border-gray-300 dark:hover:border-gray-600 rounded-md px-3 py-2 cursor-text transition-colors`}
    >
      <MarkdownView content={value} />
    </div>
  );
}
