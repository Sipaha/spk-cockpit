import { useEffect, useRef, useState } from "react";

export interface UndoToastProps {
  message: string;
  durationMs: number;
  onUndo: () => void;
  onDismiss: () => void;
}

// Toast that pops in the bottom-right with a "Revert" button and a shrinking
// progress bar at the bottom signalling how long it'll stay. Auto-dismisses
// when durationMs elapses unless the user clicks Revert.
export function UndoToast({ message, durationMs, onUndo, onDismiss }: UndoToastProps) {
  const [progress, setProgress] = useState(1);
  const dismissed = useRef(false);

  useEffect(() => {
    const start = performance.now();
    let raf = 0;
    const tick = (now: number) => {
      const left = Math.max(0, 1 - (now - start) / durationMs);
      setProgress(left);
      if (left <= 0) {
        if (!dismissed.current) {
          dismissed.current = true;
          onDismiss();
        }
        return;
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [durationMs]);

  return (
    <div className="fixed bottom-4 right-4 z-50 w-80 bg-bgsub border border-bgmute rounded shadow-2xl overflow-hidden">
      <div className="flex items-center justify-between gap-3 p-3">
        <span className="text-sm truncate">{message}</span>
        <button
          onClick={() => {
            dismissed.current = true;
            onUndo();
          }}
          className="text-accent hover:underline text-sm shrink-0"
        >
          Revert
        </button>
      </div>
      <div
        className="h-0.5 bg-accent"
        style={{ width: `${progress * 100}%`, transition: "none" }}
      />
    </div>
  );
}
