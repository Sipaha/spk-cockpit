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
import { GripVertical } from "lucide-react";

import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TodoRow } from "./TodoRow";
import type { Todo, TodoStatus } from "../lib/types";

const COLUMNS: { id: TodoStatus; label: string }[] = [
  { id: "open", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
];

type Buckets = Record<TodoStatus, Todo[]>;

function bucketize(todos: Todo[]): Buckets {
  const out: Buckets = { open: [], in_progress: [], done: [], cancelled: [] };
  for (const t of todos) {
    if (t.status in out) out[t.status].push(t);
  }
  // The list endpoint already orders by sort_order DESC, so each bucket is
  // top-to-bottom in the right order. Keep the array as-is.
  return out;
}

export function TodoBoard() {
  const {
    todos,
    loading,
    error,
    load,
    setIncludeDone,
    activeTimer,
    loadActiveTimer,
  } = useTodoStore();

  useEffect(() => {
    // The board always shows the Done column, so force-include done todos.
    setIncludeDone(true);
    void load();
    void loadActiveTimer();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const buckets = useMemo(() => bucketize(todos), [todos]);

  // Optimistic local copy so the card lands in its new column before the API
  // round-trip completes; reset whenever the server-side list refreshes.
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

    // Build the destination column without the moved card and figure out
    // where it landed (index of the card it was dropped on, or end of list
    // when the drop target was the column itself).
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
      newSortOrder = moved.sortOrder; // empty column: keep
    }

    const sameColumn = fromCol === toCol;
    const sameSlot =
      sameColumn && view[fromCol].findIndex((t) => t.id === id) === dropIndex;
    if (sameSlot && newSortOrder === moved.sortOrder) return;

    // Optimistic reorder so the card stays where it was dropped during the
    // PATCH round-trip; the SSE event handler will replace this with the
    // canonical state when it arrives.
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

  async function toggleDone(t: Todo) {
    const next: TodoStatus = t.status === "done" ? "open" : "done";
    await api.updateTodo(t.id, { status: next });
  }
  async function remove(t: Todo) {
    if (!confirm(`Delete "${t.title}"?`)) return;
    await api.deleteTodo(t.id);
  }
  async function startTimer(t: Todo) {
    await api.startTimer(t.id);
  }
  async function stopTimer() {
    await api.stopTimer();
  }
  async function renameTitle(t: Todo, title: string) {
    await api.updateTodo(t.id, { title });
  }

  const cardProps = (t: Todo) => ({
    todo: t,
    activeTimerStartedAt:
      activeTimer && activeTimer.todoId === t.id ? activeTimer.startedAt : null,
    onToggleDone: toggleDone,
    onDelete: remove,
    onStartTimer: startTimer,
    onStopTimer: stopTimer,
    onRenameTitle: renameTitle,
  });

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-xl font-semibold">Todos</h2>
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
  return (
    <div ref={setNodeRef} style={style} className="flex items-center bg-bg rounded">
      <button
        {...attributes}
        {...listeners}
        className="self-stretch px-1 text-fgmute hover:text-fg cursor-grab active:cursor-grabbing flex items-center"
        aria-label="Drag to reorder"
        type="button"
      >
        <GripVertical size={14} />
      </button>
      <div className="flex-1 min-w-0">{children}</div>
    </div>
  );
}
