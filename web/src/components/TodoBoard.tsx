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
import { Plus } from "lucide-react";

import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { parseQuickAdd } from "../lib/parser";
import { Priority } from "../lib/types";
import { TodoRow } from "./TodoRow";
import { TodoEditorModal } from "./TodoEditorModal";
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
  const out: Buckets = { open: [], in_progress: [], done: [], cancelled: [] };
  const cutoff = Math.floor(Date.now() / 1000) - DONE_VISIBLE_DAYS * 24 * 3600;
  for (const t of todos) {
    if (t.status === "done") {
      if (!t.doneAt || t.doneAt < cutoff) continue;
    }
    if (t.status in out) out[t.status].push(t);
  }
  return out;
}

type ModalState = { mode: "new" } | { mode: "edit"; todo: Todo } | null;

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
  } = useTodoStore();

  useEffect(() => {
    setIncludeDone(true);
    void load();
    void loadActiveTimer();
    void loadTags();
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
  const [undo, setUndo] = useState<Todo | null>(null);

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
  async function stopTimer(t: Todo) {
    await api.stopTimer(t.id);
  }
  function openEdit(todo: Todo) {
    setModal({ mode: "edit", todo });
  }

  // Save handler for the create modal: parse the first line for #tags,
  // !priority, due:… via the same quick-add parser the previous inline form
  // used. The cleaned title (without those tokens) becomes Todo.Title;
  // remaining lines fall through unchanged as notes.
  async function createFromModal(rawTitle: string, notes: string) {
    const parsed = parseQuickAdd(rawTitle);
    await api.createTodo({
      title: parsed.title || rawTitle,
      notes: notes || undefined,
      priority: parsed.priority ?? Priority.Normal,
      tags: parsed.tags.length > 0 ? parsed.tags : undefined,
      dueAt: parsed.dueAt,
    });
  }

  async function saveEdit(id: string, title: string, notes: string) {
    await api.updateTodo(id, { title, notes });
  }

  const cardProps = (t: Todo) => {
    const session = activeTimers.find((s) => s.todoId === t.id);
    return {
      todo: t,
      activeTimerStartedAt: session ? session.startedAt : null,
      onDelete: remove,
      onStopTimer: stopTimer,
      onEdit: openEdit,
    };
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold">Todos</h2>
        <button
          onClick={() => setModal({ mode: "new" })}
          className="flex items-center gap-1 px-3 py-1.5 bg-accent text-bg rounded text-sm hover:opacity-90"
        >
          <Plus size={14} /> Add
        </button>
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
          onClose={() => setModal(null)}
          onSave={createFromModal}
        />
      )}
      {modal?.mode === "edit" && (
        <TodoEditorModal
          heading="Edit todo"
          initialText={modal.todo.title + (modal.todo.notes ? "\n" + modal.todo.notes : "")}
          onClose={() => setModal(null)}
          onSave={(title, notes) => saveEdit(modal.todo.id, title, notes)}
        />
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
      className={`bg-bgsub rounded p-3 flex flex-col gap-2 min-h-48 transition-colors ${
        isOver ? "ring-1 ring-accent" : ""
      }`}
    >
      <h3 className="text-fgmute text-xs uppercase tracking-wide flex justify-between">
        <span>{label}</span>
        <span>{items.length}</span>
      </h3>
      <SortableContext
        items={items.map((t) => t.id)}
        strategy={verticalListSortingStrategy}
      >
        <div className="flex flex-col gap-1">
          {items.map((t) => renderCard(t))}
        </div>
      </SortableContext>
      {items.length === 0 && (
        <div className="text-fgmute text-xs text-center py-6 border border-dashed border-bgmute rounded">
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
      className="bg-bg rounded cursor-grab active:cursor-grabbing"
    >
      {children}
    </div>
  );
}
