import { useEffect, useMemo, useRef, useState } from "react";
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  pointerWithin,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import type { CollisionDetection, DragEndEvent, DragOverEvent, DragStartEvent } from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Inbox, Plus, Tag as TagIcon, Trash2 as Trash2Icon } from "lucide-react";

import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { parseQuickAdd } from "../lib/parser";
import { Priority } from "../lib/types";
import { TodoRow } from "./TodoRow";
import { TodoEditorModal } from "./TodoEditorModal";
import { TodoViewModal } from "./TodoViewModal";
import { BacklogList } from "./BacklogList";
import { TagsManager } from "./TagsManager";
import { TrashList } from "./TrashList";
import { UndoToast } from "./UndoToast";
import type { Todo, TodoStatus } from "../lib/types";
import { firstLine } from "../lib/textUtils";

const COLUMNS: { id: TodoStatus; label: string }[] = [
  { id: "open", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
];

const DONE_VISIBLE_DAYS = 3;

type Buckets = Record<TodoStatus, Todo[]>;

function bucketize(todos: Todo[]): Buckets {
  // backlog and cancelled buckets exist only to satisfy the Buckets type
  // contract; the kanban renders only open/in_progress/done.
  const out: Buckets = { open: [], in_progress: [], done: [], cancelled: [], backlog: [] };
  const cutoff = Math.floor(Date.now() / 1000) - DONE_VISIBLE_DAYS * 24 * 3600;
  for (const t of todos) {
    if (t.status === "done") {
      if (t.dismissedAt) continue;
      if (!t.doneAt || t.doneAt < cutoff) continue;
    }
    if (t.status in out) out[t.status].push(t);
  }
  // We can't trust the incoming todos array's order: SSE updates replace a
  // todo in place without reshuffling, so a sortOrder change leaves the
  // array stale (the moved card stays in the OLD slot of the array). Sort
  // each visible bucket the same way the SQL ORDER BY does it.
  const sortBucket = (xs: Todo[]) =>
    xs.sort((a, b) =>
      b.sortOrder !== a.sortOrder
        ? b.sortOrder - a.sortOrder
        : b.createdAt - a.createdAt,
    );
  sortBucket(out.open);
  sortBucket(out.in_progress);
  sortBucket(out.done);
  return out;
}

type ModalState =
  | { mode: "new" }
  | { mode: "view"; todo: Todo }
  | { mode: "edit"; todo: Todo }
  | null;

export function TodoBoard() {
  // Per-field selectors so unrelated state slices (meetings, syncStates,
  // etc.) mutating from SSE events don't re-render the entire kanban.
  const todos = useTodoStore((s) => s.todos);
  const loading = useTodoStore((s) => s.loading);
  const error = useTodoStore((s) => s.error);
  const setIncludeDone = useTodoStore((s) => s.setIncludeDone);
  const activeTimers = useTodoStore((s) => s.activeTimers);
  const loadActiveTimer = useTodoStore((s) => s.loadActiveTimer);
  const loadTags = useTodoStore((s) => s.loadTags);
  const loadTaskPatterns = useTodoStore((s) => s.loadTaskPatterns);
  const taskPatterns = useTodoStore((s) => s.taskPatterns);

  // Initial fetch — runs once on mount. Zustand actions are stable references,
  // so listing them as deps would just re-fire the effect on every selector
  // change with no benefit; the empty array is intentional.
  // setIncludeDone() in the store already triggers load(), so we don't call
  // load() explicitly to avoid two concurrent /api/todos requests.
  useEffect(() => {
    setIncludeDone(true);
    void loadActiveTimer();
    void loadTags();
    void loadTaskPatterns();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const buckets = useMemo(() => bucketize(todos), [todos]);

  // Optimistic-state model: `override` shows the pending drop layout for one
  // in-flight move. `pendingMove` records what we asked the server for so we
  // can (a) detect when the SSE echo for the move arrives (then the override
  // can clear) and (b) ignore unrelated `todos` updates that would otherwise
  // wipe the override mid-drag, and (c) avoid a failing move's catch block
  // from clearing a SECOND, still-in-flight drag's optimistic state.
  const [override, setOverride] = useState<Buckets | null>(null);
  type PendingMove = { seq: number; id: string; targetStatus: TodoStatus };
  const [pendingMove, setPendingMove] = useState<PendingMove | null>(null);
  const moveSeqRef = useRef(0);

  useEffect(() => {
    if (!override || !pendingMove) return;
    // Clear the override only when the SSE-driven todos array reflects the
    // move we kicked off — i.e. the moved todo is now in its target column
    // server-side. Unrelated SSE events (timer started, sibling edited) leave
    // the optimistic state intact.
    const moved = todos.find((t) => t.id === pendingMove.id);
    if (moved && moved.status === pendingMove.targetStatus) {
      setOverride(null);
      setPendingMove(null);
    }
  }, [todos, override, pendingMove]);
  const view = override ?? buckets;

  // activeSnapshot is the dragged todo captured at onDragStart so DragOverlay
  // is immune to concurrent SSE updates that might otherwise change the
  // overlay's title/tags mid-drag while the underlying SortableCard still
  // shows pre-drag state. (The SortableCard's own `isDragging` style hook
  // comes from dnd-kit, so we don't need to track activeId separately.)
  const [activeSnapshot, setActiveSnapshot] = useState<Todo | null>(null);

  const [modal, setModal] = useState<ModalState>(null);
  const [tagsModalOpen, setTagsModalOpen] = useState(false);
  const [trashModalOpen, setTrashModalOpen] = useState(false);
  const [backlogModalOpen, setBacklogModalOpen] = useState(false);
  const [undo, setUndo] = useState<Todo | null>(null);
  // Reading s.tags directly keeps the selector referentially stable; mapping
  // to names runs in a memo so we don't trigger Zustand's "snapshot changed
  // every render" infinite-loop guard.
  const tagsRaw = useTodoStore((s) => s.tags);
  const tagNames = useMemo(() => tagsRaw.map((t) => t.name), [tagsRaw]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );

  // Custom collision detection. closestCorners (the v0 strategy) reports the
  // numerically-closest droppable corner, which for cross-column drag picks
  // the wrong card when the cursor sits in column body well below all cards
  // — the last card's bottom corner stays "closest" enough that the column
  // never wins, so "drop at end" becomes "drop at second-to-last".
  // pointerWithin is precise: it only returns droppables whose rect contains
  // the cursor. With overlapping droppables (cards inside their column),
  // we prefer the card hit (more specific) and fall back to the column when
  // the cursor is in column body only. closestCorners stays as a fallback
  // for edges where the cursor is briefly outside any droppable rect.
  const collisionDetection: CollisionDetection = (args) => {
    const pointerCollisions = pointerWithin(args);
    if (pointerCollisions.length > 0) {
      const colIds = new Set<string>(["open", "in_progress", "done"]);
      const cardCollisions = pointerCollisions.filter((c) => !colIds.has(String(c.id)));
      return cardCollisions.length > 0 ? cardCollisions : pointerCollisions;
    }
    return closestCorners(args);
  };

  function findContainer(id: string): TodoStatus | null {
    if (id === "open" || id === "in_progress" || id === "done") return id;
    for (const status of ["open", "in_progress", "done"] as const) {
      if (view[status].some((t) => t.id === id)) return status;
    }
    return null;
  }

  function onDragStart(e: DragStartEvent) {
    const id = String(e.active.id);
    setActiveSnapshot(todos.find((t) => t.id === id) ?? null);
  }

  // Multi-container sortable pattern (per dnd-kit's "Multiple Containers"
  // example). dnd-kit's verticalListSortingStrategy reorders cards
  // visually via CSS transforms WITHIN a SortableContext, but it doesn't
  // mutate our items array. For our after/before-id move API to see the
  // user's true drop position, we keep `override` (and therefore `view`)
  // in sync with the live drag — both for cross-column relocations and
  // for same-column reorders. onDragEnd just reads the final view[toCol]
  // and computes afterId/beforeId from the now-authoritative ordering.
  function onDragOver(e: DragOverEvent) {
    const { active, over } = e;
    if (!over) return;

    const activeId = String(active.id);
    const overId = String(over.id);
    if (activeId === overId) return;

    setOverride((current) => {
      // Build the working buckets from CURRENT (latest committed override),
      // not from `view` (closure-captured, may be one render behind when
      // dnd-kit fires events faster than React commits).
      const buckets: Buckets = current ?? {
        open: bucketize(todos).open,
        in_progress: bucketize(todos).in_progress,
        done: bucketize(todos).done,
        cancelled: [],
        backlog: [],
      };

      // Re-compute containers against the latest buckets — they reflect the
      // running override, so previous onDragOver relocations are visible.
      const findIn = (id: string): TodoStatus | null => {
        if (id === "open" || id === "in_progress" || id === "done") return id;
        for (const status of ["open", "in_progress", "done"] as const) {
          if (buckets[status].some((t) => t.id === id)) return status;
        }
        return null;
      };
      const activeContainer = findIn(activeId);
      const overContainer = findIn(overId);
      if (!activeContainer || !overContainer) return current;

      if (activeContainer === overContainer) {
        // Same-column reorder is handled by dnd-kit's
        // verticalListSortingStrategy (CSS transforms only) plus our
        // onDragEnd's final arrayMove. Mutating override here causes an
        // infinite re-render loop: the arrayMove flips active/over
        // positions, dnd-kit fires another onDragOver against the new
        // layout, we arrayMove back, and so on.
        return current;
      }

      // Cross-column relocate.
      const sourceItems = buckets[activeContainer];
      const targetItems = buckets[overContainer];
      const activeIndex = sourceItems.findIndex((t) => t.id === activeId);
      if (activeIndex < 0) return current;

      const movedItem = { ...sourceItems[activeIndex], status: overContainer };

      // Insert at the over-card's index by default (matches dnd-kit's
      // verticalListSortingStrategy visual). Special-case "dragged FULLY
      // past the over card" — closestCorners keeps reporting the last
      // card as `over` even when the cursor is well below it, so without
      // this check `drop into empty column space` lands on the
      // second-to-last slot. Threshold is the over card's BOTTOM edge
      // (active.top > over.bottom) so middle-of-card hovers stay "before".
      let newIndex: number;
      if (overId === overContainer) {
        newIndex = targetItems.length;
      } else {
        const overIndex = targetItems.findIndex((t) => t.id === overId);
        if (overIndex < 0) {
          newIndex = targetItems.length;
        } else {
          const activeTranslated = active.rect.current.translated;
          const overRect = over.rect;
          const isPast =
            activeTranslated && overRect
              ? activeTranslated.top > overRect.top + overRect.height
              : false;
          newIndex = overIndex + (isPast ? 1 : 0);
        }
      }

      return {
        ...buckets,
        [activeContainer]: sourceItems.filter((t) => t.id !== activeId),
        [overContainer]: [
          ...targetItems.slice(0, newIndex),
          movedItem,
          ...targetItems.slice(newIndex),
        ],
      };
    });
  }

  async function onDragEnd(e: DragEndEvent) {
    setActiveSnapshot(null);
    const { active, over } = e;
    if (!over) {
      // Drop outside any droppable — discard the in-flight visual override.
      setOverride(null);
      return;
    }

    const activeId = String(active.id);
    const moved = todos.find((t) => t.id === activeId);
    if (!moved) {
      setOverride(null);
      return;
    }
    const fromCol = moved.status as TodoStatus;
    const toCol = findContainer(activeId);
    if (!toCol) {
      setOverride(null);
      return;
    }
    const sameColumn = fromCol === toCol;

    // Compute the final post-drop list. For cross-column, onDragOver already
    // placed the card in view[toCol]; we re-position it via arrayMove using
    // the final overId so the drop matches the user's last cursor position
    // (closer than the last onDragOver fire). For same-column, view[toCol]
    // == bucketize(todos)[toCol] (no override changes) so arrayMove operates
    // on the original ordering.
    const baseList = view[toCol].slice();
    const oldIndex = baseList.findIndex((t) => t.id === activeId);
    if (oldIndex < 0) {
      setOverride(null);
      return;
    }
    const overId = String(over.id);
    let newIndex: number;
    if (overId === toCol) {
      newIndex = baseList.length - 1;
    } else if (overId === activeId) {
      newIndex = oldIndex;
    } else {
      const overIdx = baseList.findIndex((t) => t.id === overId);
      if (overIdx < 0) {
        newIndex = oldIndex;
      } else {
        // For arrayMove convention: target index is over's index. Account for
        // "dragged fully past over" as one slot further.
        const activeTranslated = active.rect.current.translated;
        const overRect = over.rect;
        const isPast =
          activeTranslated && overRect
            ? activeTranslated.top > overRect.top + overRect.height
            : false;
        newIndex = isPast ? overIdx + 1 : overIdx;
        // Clamp — arrayMove past length-1 has no effect.
        if (newIndex > baseList.length - 1) newIndex = baseList.length - 1;
      }
    }

    if (sameColumn && oldIndex === newIndex) {
      setOverride(null);
      return;
    }

    const postList = arrayMove(baseList, oldIndex, newIndex);
    const idx = postList.findIndex((t) => t.id === activeId);
    const afterId = idx > 0 ? postList[idx - 1].id : undefined;
    const beforeId = idx < postList.length - 1 ? postList[idx + 1].id : undefined;

    // Persist `postList` as the optimistic override so it survives the await
    // below. For same-column we just write the reordered postList. For
    // cross-column view[fromCol] already has the card removed (onDragOver
    // did that), and view[toCol] now becomes postList.
    const next: Buckets = {
      open: view.open.slice(),
      in_progress: view.in_progress.slice(),
      done: view.done.slice(),
      cancelled: view.cancelled,
      backlog: view.backlog,
    };
    next[toCol] = postList;
    setOverride(next);
    const seq = ++moveSeqRef.current;
    setPendingMove({ seq, id: activeId, targetStatus: toCol });

    try {
      await api.moveTodo(activeId, {
        afterId,
        beforeId,
        status: sameColumn ? undefined : toCol,
      });
    } catch (err) {
      // Surface the failure so a stale daemon binary (no /move endpoint) or
      // a 4xx from the server doesn't just look like a silent revert. Only
      // revert if no later move has superseded this one — otherwise we'd
      // wipe a still-in-flight drag's optimistic state.
      console.error("[todo move] request failed, reverting card", err);
      if (moveSeqRef.current === seq) {
        setOverride(null);
        setPendingMove(null);
      }
    }
  }

  async function remove(t: Todo) {
    await api.deleteTodo(t.id);
    setUndo(t);
  }
  async function undoDelete(t: Todo) {
    setUndo(null);
    try {
      await api.restoreTodo(t.id);
    } catch {
      // ignore — restore failure leaves the toast dismissed; user can find it in Trash
    }
  }
  function openView(todo: Todo) {
    setModal({ mode: "view", todo });
  }
  function openEdit(todo: Todo) {
    setModal({ mode: "edit", todo });
  }
  async function hideFromDone(todo: Todo) {
    try {
      await api.dismissTodo(todo.id);
    } catch {
      // ignore — failure leaves the card visible; user can retry.
    }
  }
  async function sendToBacklog(todo: Todo) {
    try {
      await api.updateTodo(todo.id, { status: "backlog" });
    } catch {
      // ignore — SSE event would have echoed success; failures are usually
      // transient and the user can retry.
    }
  }

  // Save handler for the create modal: parse the first line for #tags,
  // !priority, due:… via the same quick-add parser the previous inline form
  // used. The cleaned title (without those tokens) becomes Todo.Title;
  // remaining lines fall through unchanged as notes. Tags entered explicitly
  // via TagInput are merged with whatever the parser pulled out of the
  // title, so both pathways add up. The modal-selected priority overrides
  // the parsed one when the user touched the priority picker (i.e. it's
  // not the default Normal); otherwise the parsed tag wins.
  async function createFromModal(
    rawTitle: string,
    notes: string,
    tags: string[],
    priority: Priority,
  ) {
    const parsed = parseQuickAdd(rawTitle);
    const merged = Array.from(new Set([...parsed.tags, ...tags]));
    const finalPriority =
      priority !== Priority.Normal
        ? priority
        : parsed.priority ?? Priority.Normal;
    await api.createTodo({
      title: parsed.title || rawTitle,
      notes: notes || undefined,
      priority: finalPriority,
      tags: merged.length > 0 ? merged : undefined,
      dueAt: parsed.dueAt,
    });
  }

  async function saveEdit(
    id: string,
    title: string,
    notes: string,
    tags: string[],
    priority: Priority,
  ) {
    await api.updateTodo(id, { title, notes, tags, priority });
  }

  const cardProps = (t: Todo) => {
    const session = activeTimers.find((s) => s.todoId === t.id);
    return {
      todo: t,
      activeTimerStartedAt: session ? session.startedAt : null,
      taskPatterns,
      onDelete: remove,
      onView: openView,
      onEdit: openEdit,
      onHide: t.status === "done" ? hideFromDone : undefined,
      onBacklog: t.status === "open" ? sendToBacklog : undefined,
    };
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-semibold">Todos</h2>
          <button
            onClick={() => setModal({ mode: "new" })}
            className="flex items-center gap-1 px-3 py-1.5 bg-accent text-bg rounded text-sm hover:opacity-90"
          >
            <Plus size={14} /> Add
          </button>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setBacklogModalOpen(true)}
            title="Open backlog"
            className="flex items-center gap-1 px-3 py-1.5 text-fgmute hover:text-fg rounded text-sm border border-bgmute hover:border-fgmute"
          >
            <Inbox size={14} /> Backlog
          </button>
          <button
            onClick={() => setTagsModalOpen(true)}
            title="Manage tags"
            className="flex items-center gap-1 px-3 py-1.5 text-fgmute hover:text-fg rounded text-sm border border-bgmute hover:border-fgmute"
          >
            <TagIcon size={14} /> Tags
          </button>
          <button
            onClick={() => setTrashModalOpen(true)}
            title="Open trashcan"
            className="flex items-center gap-1 px-3 py-1.5 text-fgmute hover:text-fg rounded text-sm border border-bgmute hover:border-fgmute"
          >
            <Trash2Icon size={14} /> Trashcan
          </button>
        </div>
      </div>
      {loading && <div className="text-fgmute">loading…</div>}
      {error && <div className="text-urgent">error: {error}</div>}
      <DndContext
        sensors={sensors}
        collisionDetection={collisionDetection}
        onDragStart={onDragStart}
        onDragOver={onDragOver}
        onDragEnd={onDragEnd}
      >
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {COLUMNS.map((col) => (
            <Column
              key={col.id}
              id={col.id}
              label={col.label}
              items={view[col.id]}
              renderCard={(t) => (
                <SortableCard key={t.id} todo={t}>
                  <TodoRow {...cardProps(t)} />
                </SortableCard>
              )}
            />
          ))}
        </div>
        {/*
          dropAnimation={null} disables dnd-kit's default 250ms transition
          that animates the overlay back to the SOURCE element's bounding
          box on release. Since onDragEnd applies an optimistic override
          that places the card in the TARGET column synchronously, the
          default animation looks like "card flies back to origin, then
          jumps to target" — confusing. With null, the overlay vanishes
          instantly and the optimistic re-render is what the user sees.
        */}
        <DragOverlay dropAnimation={null}>
          {activeSnapshot && (
            <div className="bg-bg border border-accent rounded shadow-lg">
              <TodoRow {...cardProps(activeSnapshot)} />
            </div>
          )}
        </DragOverlay>
      </DndContext>
      {modal?.mode === "new" && (
        <TodoEditorModal
          heading="New todo"
          initialText=""
          initialTags={[]}
          initialPriority={Priority.Normal}
          tagSuggestions={tagNames}
          onClose={() => setModal(null)}
          onSave={createFromModal}
        />
      )}
      {modal?.mode === "view" && (
        <TodoViewModal
          todo={modal.todo}
          taskPatterns={taskPatterns}
          onClose={() => setModal(null)}
          onEdit={() => setModal({ mode: "edit", todo: modal.todo })}
        />
      )}
      {modal?.mode === "edit" && (
        <TodoEditorModal
          heading="Edit todo"
          initialText={modal.todo.title + (modal.todo.notes ? "\n" + modal.todo.notes : "")}
          initialTags={modal.todo.tags ?? []}
          initialPriority={modal.todo.priority}
          tagSuggestions={tagNames}
          onClose={() => setModal(null)}
          onSave={(title, notes, tags, priority) =>
            saveEdit(modal.todo.id, title, notes, tags, priority)
          }
        />
      )}
      {tagsModalOpen && (
        <div className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6">
          <div className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-lg flex flex-col gap-3 p-4">
            <div className="flex items-center justify-between">
              <div className="text-fgmute text-xs uppercase tracking-wide">Manage tags</div>
              <button
                onClick={() => setTagsModalOpen(false)}
                className="text-fgmute hover:text-fg text-sm"
              >
                Close
              </button>
            </div>
            <TagsManager />
          </div>
        </div>
      )}
      {backlogModalOpen && (
        <div className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6">
          <div className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-lg flex flex-col gap-3 p-4">
            <div className="flex items-center justify-between">
              <div className="text-fgmute text-xs uppercase tracking-wide">Backlog</div>
              <button
                onClick={() => setBacklogModalOpen(false)}
                className="text-fgmute hover:text-fg text-sm"
              >
                Close
              </button>
            </div>
            <p className="text-fgmute text-sm">
              Parked todos. Click the arrow to promote one back to To Do.
            </p>
            <BacklogList />
          </div>
        </div>
      )}
      {trashModalOpen && (
        <div className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6">
          <div className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-lg flex flex-col gap-3 p-4">
            <div className="flex items-center justify-between">
              <div className="text-fgmute text-xs uppercase tracking-wide">Trashcan</div>
              <button
                onClick={() => setTrashModalOpen(false)}
                className="text-fgmute hover:text-fg text-sm"
              >
                Close
              </button>
            </div>
            <p className="text-fgmute text-sm">
              Recently deleted todos. Click Restore to bring one back to the board.
            </p>
            <TrashList />
          </div>
        </div>
      )}
      {undo && (
        <UndoToast
          message={`Deleted "${firstLine(undo.title, 60)}"`}
          durationMs={6000}
          onUndo={() => void undoDelete(undo)}
          onDismiss={() => setUndo(null)}
        />
      )}
    </div>
  );
}

interface ColumnProps {
  id: TodoStatus;
  label: string;
  items: Todo[];
  renderCard: (t: Todo) => React.ReactNode;
}

function Column({ id, label, items, renderCard }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id });
  return (
    <div
      ref={setNodeRef}
      className={`bg-bgsub rounded-lg p-3 flex flex-col gap-3 min-h-48 transition-colors ${
        isOver ? "ring-1 ring-accent" : ""
      }`}
    >
      <h3 className="text-fgmute text-[11px] uppercase tracking-wider font-semibold flex justify-between items-center px-1">
        <span>{label}</span>
        <span className="text-fgmute/70 text-xs font-normal">{items.length}</span>
      </h3>
      <SortableContext
        items={items.map((t) => t.id)}
        strategy={verticalListSortingStrategy}
      >
        <div className="flex flex-col gap-2">
          {items.map((t) => renderCard(t))}
        </div>
      </SortableContext>
      {items.length === 0 && (
        <div className="text-fgmute/70 text-xs text-center py-8 border border-dashed border-bgmute/60 rounded-md">
          drop here
        </div>
      )}
    </div>
  );
}

interface SortableCardProps {
  todo: Todo;
  children: React.ReactNode;
}

function SortableCard({ todo, children }: SortableCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: todo.id });
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };
  // Listeners are spread on the wrapper so the entire card body acts as a
  // drag handle. PointerSensor's distance: 6 activation lets a click fall
  // through to nested buttons / the title-click edit handler instead of
  // starting a drag — drags only kick in once the pointer has moved.
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className="bg-bg rounded-md border border-bgmute hover:border-fgmute hover:shadow-md transition-[border-color,box-shadow] cursor-grab active:cursor-grabbing overflow-hidden"
    >
      {children}
    </div>
  );
}
