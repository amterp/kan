import { useState, useRef, useEffect } from 'react';
import MarkdownView from './MarkdownView';

interface MarkdownFieldProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  minHeight?: string;
}

export default function MarkdownField({
  value,
  onChange,
  placeholder = '',
  minHeight = 'min-h-48',
}: MarkdownFieldProps) {
  const [isEditing, setIsEditing] = useState(!value?.trim());
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (isEditing && textareaRef.current) {
      textareaRef.current.focus();
      const len = textareaRef.current.value.length;
      textareaRef.current.setSelectionRange(len, len);
    }
  }, [isEditing]);

  const handleBlur = () => {
    if (value?.trim()) {
      setIsEditing(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Escape' && value?.trim()) {
      e.preventDefault();
      setIsEditing(false);
      return;
    }

    // Cmd+Enter exits edit mode (stops propagation so modal doesn't save)
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      if (value?.trim()) {
        e.preventDefault();
        e.nativeEvent.stopImmediatePropagation();
        setIsEditing(false);
      }
      return;
    }

    // Formatting shortcuts
    const textarea = e.currentTarget;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selectedText = value.substring(start, end);

    const wrapSelection = (prefix: string, suffix: string = prefix) => {
      e.preventDefault();
      const before = value.substring(0, start);
      const after = value.substring(end);
      const newValue = before + prefix + selectedText + suffix + after;
      onChange(newValue);
      // Restore selection after React re-renders
      requestAnimationFrame(() => {
        textarea.setSelectionRange(start + prefix.length, end + prefix.length);
      });
    };

    if (e.metaKey || e.ctrlKey) {
      switch (e.key.toLowerCase()) {
        case 'b':
          wrapSelection('**');
          break;
        case 'i':
          wrapSelection('_');
          break;
        case 'k':
          // Link: wrap in [text](url)
          e.preventDefault();
          if (selectedText) {
            const before = value.substring(0, start);
            const after = value.substring(end);
            const newValue = before + '[' + selectedText + '](url)' + after;
            onChange(newValue);
            requestAnimationFrame(() => {
              // Select "url" for easy replacement
              textarea.setSelectionRange(end + 3, end + 6);
            });
          }
          break;
      }
    }
  };

  if (isEditing) {
    return (
      <textarea
        ref={textareaRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onBlur={handleBlur}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        className={`w-full ${minHeight} border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none`}
      />
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
