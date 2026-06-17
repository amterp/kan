/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useCallback } from 'react';
import type { ReactNode } from 'react';

export type ToastType = 'error' | 'success' | 'info';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
}

interface ToastContextValue {
  toasts: Toast[];
  showToast: (type: ToastType, message: string) => void;
  dismissToast: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

// Auto-dismiss timing scales with reading time so long messages aren't rushed
// off-screen and short ones don't linger. BASE is "time to notice the toast"
// before reading begins; MS_PER_WORD is an unhurried glance-read (~170 wpm).
// MIN/MAX keep both extremes sane.
const TOAST_MIN_DURATION = 4000;
const TOAST_MAX_DURATION = 10000;
const TOAST_BASE_DURATION = 2000;
const TOAST_MS_PER_WORD = 350;

function durationForMessage(message: string): number {
  const words = message.trim().split(/\s+/).filter(Boolean).length;
  const estimate = TOAST_BASE_DURATION + words * TOAST_MS_PER_WORD;
  return Math.min(TOAST_MAX_DURATION, Math.max(TOAST_MIN_DURATION, estimate));
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const showToast = useCallback((type: ToastType, message: string) => {
    const id = `toast-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
    const toast: Toast = { id, type, message };

    setToasts((prev) => [...prev, toast]);

    // Auto-dismiss after a reading-time-scaled delay.
    setTimeout(() => {
      dismissToast(id);
    }, durationForMessage(message));
  }, [dismissToast]);

  return (
    <ToastContext.Provider value={{ toasts, showToast, dismissToast }}>
      {children}
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
