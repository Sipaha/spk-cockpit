import { Todos } from "./pages/Todos";

export function App() {
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">spk-cockpit</h1>
        <nav className="flex flex-col gap-1 text-fgmute">
          <a className="text-fg">Todos</a>
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Todos />
      </main>
    </div>
  );
}
