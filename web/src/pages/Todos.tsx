import { useEffect } from "react";
import { TodoBoard } from "../components/TodoBoard";
import { AddTodoForm } from "../components/AddTodoForm";
import { EventStream } from "../lib/events";
import { useTodoStore } from "../lib/store";

const stream = new EventStream();

export function Todos() {
  const applyEvent = useTodoStore((s) => s.applyEvent);

  useEffect(() => {
    stream.start();
    const off = stream.on(applyEvent);
    return () => {
      off();
      stream.stop();
    };
  }, [applyEvent]);

  return (
    <div className="flex flex-col gap-6">
      <AddTodoForm />
      <TodoBoard />
    </div>
  );
}
