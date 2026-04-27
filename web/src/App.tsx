import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import { Todos } from "./pages/Todos";
import { Popover } from "./pages/Popover";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/popover" element={<Popover />} />
        <Route path="*" element={<MainShell />} />
      </Routes>
    </BrowserRouter>
  );
}

function MainShell() {
  const loc = useLocation();
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">spk-cockpit</h1>
        <nav className="flex flex-col gap-1 text-fgmute">
          <Link to="/" className={loc.pathname === "/" ? "text-fg" : ""}>
            Todos
          </Link>
          <Link to="/popover" className="text-fgmute">
            Compact view
          </Link>
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Todos />
      </main>
    </div>
  );
}
