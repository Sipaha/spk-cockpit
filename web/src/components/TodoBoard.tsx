import { useEffect, useMemo, useState } from "react";
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import type { DragEndEvent, DragStartEvent } from "@dnd-kit/core";
import {
  SortableContext,
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
  return out;
}

type ModalState =
  | { mode: "new" }
  | { mode: "view"; todo: Todo }
  | { mode: "edit"; todo: Todo }
  | null;

export function TodoBoard() {
  const {
    todos,
    loading,
    error,
    load,
    setIncludeDone,
    activeTimers,
    loadActiveTimer,
    loadTags,
    loadTaskPatterns,
    taskPatterns,
  } = useTodoStore();

  useEffect(() => {
    setIncludeDone(true);
    void load();
    void loadActiveTimer();
    void loadTags();
    void loadTaskPatterns();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const buckets = useMemo(() => bucketize(todos), [todos]);

  const [override, setOverride] = useState<Buckets | null>(null);
  useEffect(() => {
    setOverride(null);
  }, [todos]);
  const view = override ?? buckets;

  const [activeId, setActiveId] = useState<string | null>(null);
  const activeTodo = useMemo(
    () => todos.find((t) => t.id === activeId) ?? null,
    [activeId, todos],
  );

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

  function findContainer(id: string): TodoStatus | null {
    if (id === "open" || id === "in_progress" || id === "done") return id;
    for (const status of ["open", "in_progress", "done"] as const) {
      if (view[status].some((t) => t.id === id)) return status;
    }
    return null;
  }

  function onDragStart(e: DragStartEvent) {
    setActiveId(String(e.active.id));
  }

  async function onDragEnd(e: DragEndEvent) {
    setActiveId(null);
    const id = String(e.active.id);
    const overId = e.over ? String(e.over.id) : null;
    if (!overId) return;

    const fromCol = findContainer(id);
    const toCol = findContainer(overId);
    if (!fromCol || !toCol) return;

    const moved = todos.find((t) => t.id === id);
    if (!moved) return;

    const dest = view[toCol].filter((t) => t.id !== id);
    let dropIndex = dest.length;
    if (overId !== toCol) {
      const i = dest.findIndex((t) => t.id === overId);
      if (i >= 0) dropIndex = i;
    }

    const above = dest[dropIndex - 1];
    const below = dest[dropIndex];
    let newSortOrder: number;
    if (above && below) {
      newSortOrder = (above.sortOrder + below.sortOrder) / 2;
    } else if (above) {
      newSortOrder = above.sortOrder - 1;
    } else if (below) {
      newSortOrder = below.sortOrder + 1;
    } else {
      newSortOrder = moved.sortOrder;
    }

    const sameColumn = fromCol === toCol;
    const sameSlot =
      sameColumn && view[fromCol].findIndex((t) => t.id === id) === dropIndex;
    if (sameSlot && newSortOrder === moved.sortOrder) return;

    const next: Buckets = {
      open: view.open.slice(),
      in_progress: view.in_progress.slice(),
      done: view.done.slice(),
      cancelled: view.cancelled,
      backlog: view.backlog,
    };
    next[fromCol] = next[fromCol].filter((t) => t.id !== id);
    const updated: Todo = { ...moved, status: toCol, sortOrder: newSortOrder };
    next[toCol].splice(dropIndex, 0, updated);
    setOverride(next);

    try {
      const patch: { status?: TodoStatus; sortOrder?: number } = {
        sortOrder: newSortOrder,
      };
      if (!sameColumn) patch.status = toCol;
      // dismissedAt clears server-side when the status moves out of Done
      // (see Service.Update); no client bookkeeping needed here.
      await api.updateTodo(id, patch);
    } catch {
      setOverride(null);
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
  // title, so both pathways add up.
  async function createFromModal(rawTitle: string, notes: string, tags: string[]) {
    const parsed = parseQuickAdd(rawTitle);
    const merged = Array.from(new Set([...parsed.tags, ...tags]));
    await api.createTodo({
      title: parsed.title || rawTitle,
      notes: notes || undefined,
      priority: parsed.priority ?? Priority.Normal,
      tags: merged.length > 0 ? merged : undefined,
      dueAt: parsed.dueAt,
    });
  }

  async function saveEdit(id: string, title: string, notes: string, tags: string[]) {
    await api.updateTodo(id, { title, notes, tags });
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
        collisionDetection={closestCorners}
        onDragStart={onDragStart}
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
        <DragOverlay>
          {activeTodo && (
            <div className="bg-bg border border-accent rounded shadow-lg">
              <TodoRow {...cardProps(activeTodo)} />
            </div>
          )}
        </DragOverlay>
      </DndContext>
      {modal?.mode === "new" && (
        <TodoEditorModal
          heading="New todo"
          initialText=""
          initialTags={[]}
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
          tagSuggestions={tagNames}
          onClose={() => setModal(null)}
          onSave={(title, notes, tags) => saveEdit(modal.todo.id, title, notes, tags)}
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

function firstLine(s: string, max: number): string {
  const nl = s.indexOf("\n");
  const line = nl === -1 ? s : s.slice(0, nl);
  return line.length > max ? line.slice(0, max) + "…" : line;
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
