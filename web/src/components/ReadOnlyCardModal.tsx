import { useEffect, useRef, useState } from 'react';
import { getLinkedCard } from '../api/cards';
import type { LinkedCardResponse } from '../api/types';
import { BoardConfigProvider } from '../contexts/BoardConfigContext';
import MarkdownView from './MarkdownView';
import CardLinkList from './CardLinkList';

interface ReadOnlyCardModalProps {
  cardRef: string;
  onOpenCard: (ref: string) => void;
  onClose: () => void;
}

export default function ReadOnlyCardModal({ cardRef, onOpenCard, onClose }: ReadOnlyCardModalProps) {
  const [result, setResult] = useState<{ ref: string; data?: LinkedCardResponse; error?: string }>();
  const dialog = useRef<HTMLDivElement>(null);
  const backdrop = useRef(false);
  useEffect(() => {
    let canceled = false;
    getLinkedCard(cardRef).then(
      (data) => { if (!canceled) setResult({ ref: cardRef, data }); },
      (error) => { if (!canceled) setResult({ ref: cardRef, error: error instanceof Error ? error.message : 'Unable to read card' }); },
    );
    return () => { canceled = true; };
  }, [cardRef]);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    dialog.current?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); onClose(); }
    };
    window.addEventListener('keydown', keydown);
    return () => { window.removeEventListener('keydown', keydown); previous?.focus(); };
  }, [onClose]);
  const current = result?.ref === cardRef ? result : undefined;
  const data = current?.data;
  const card = data?.result === 'found' ? data.card : undefined;
  const board = data?.board_config;
  const links = data?.links;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onMouseDown={(event) => { backdrop.current = event.target === event.currentTarget; }}
      onClick={(event) => { if (backdrop.current && event.target === event.currentTarget) onClose(); }}>
      <div ref={dialog} role="dialog" aria-modal="true" aria-label={card?.title ?? 'Linked card'} tabIndex={-1}
        className="w-full max-w-4xl max-h-[85vh] overflow-y-auto rounded-xl bg-white dark:bg-gray-800 shadow-2xl text-gray-900 dark:text-white p-6">
        <div className="flex items-start justify-between gap-4 mb-4">
          <div>
            <p className="text-sm text-gray-500 dark:text-gray-400" title={data?.project_path}>
              Read only{data?.project_name ? ` · ${data.project_name}` : ''}{data?.board ? ` / ${data.board}` : ''}
            </p>
            <h2 className="text-xl font-semibold">{card?.title ?? cardRef}</h2>
          </div>
          <button type="button" onClick={onClose} className="px-3 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700">Close</button>
        </div>
        {!current && <p role="status">Loading card...</p>}
        {current?.error && <p role="alert">{current.error}</p>}
        {data && !card && <CardLinkList label="Unavailable card" links={[data]} onOpen={onOpenCard} />}
        {card && board && (
          <BoardConfigProvider board={board}>
            <div className="grid gap-6 md:grid-cols-[1fr_16rem]">
              <div>
                {card.description && <MarkdownView content={card.description} />}
                {(card.comments?.length ?? 0) > 0 && <section className="mt-6" aria-label="Comments">
                  <h3 className="font-medium mb-2">Comments</h3>
                  {card.comments?.map((comment) => <div key={comment.id} className="border-t border-gray-200 dark:border-gray-700 py-3">
                    <p className="text-sm text-gray-500 dark:text-gray-400">{comment.author}</p>
                    <MarkdownView content={comment.body} />
                  </div>)}
                </section>}
              </div>
              <aside>
                <p className="mb-4"><span className="text-sm text-gray-500 dark:text-gray-400">Column</span><br />{card.column}</p>
                <CardLinkList label="Blocked by" links={(card.blocked_by ?? []).map((ref) => links?.resolved[ref] ?? { ref, result: 'unreadable' })} onOpen={onOpenCard} />
                <CardLinkList label="Blocks" links={links?.blocks[card.id] ?? []} onOpen={onOpenCard} />
                <CardLinkList label="Related" links={(card.related ?? []).map((ref) => links?.resolved[ref] ?? { ref, result: 'unreadable' })} onOpen={onOpenCard} />
                <dl className="space-y-3 text-sm">
                  {Object.keys(board.custom_fields ?? {}).filter((name) => card[name] !== undefined).map((name) => (
                    <div key={name}><dt className="text-gray-500 dark:text-gray-400">{name}</dt>
                      <dd className="break-words">{typeof card[name] === 'object' ? JSON.stringify(card[name]) : String(card[name])}</dd></div>
                  ))}
                </dl>
              </aside>
            </div>
          </BoardConfigProvider>
        )}
      </div>
    </div>
  );
}
