import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import { Todos } from "./pages/Todos";
import { Popover } from "./pages/Popover";
import { Calendar } from "./pages/Calendar";
import { Settings } from "./pages/Settings";
import { Standup } from "./pages/Standup";
import { MeetingPopup } from "./pages/MeetingPopup";
import { QuickAddTodo } from "./pages/QuickAddTodo";
import { Trash } from "./pages/Trash";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/popover" element={<Popover />} />
        <Route path="/popup-meeting" element={<MeetingPopup />} />
        <Route path="/quick-add-todo" element={<QuickAddTodo />} />
        <Route path="*" element={<MainShell />} />
      </Routes>
    </BrowserRouter>
  );
}

function MainShell() {
  const loc = useLocation();
  const navItem = (to: string, label: string) => {
    const active = loc.pathname === to;
    return (
      <Link
        to={to}
        className={`-mx-2 block rounded px-3 py-2 text-sm transition-colors ${
          active
            ? "bg-bgmute text-fg"
            : "text-fgmute hover:bg-bgmute hover:text-fg"
        }`}
      >
        {label}
      </Link>
    );
  };
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">SPK Cockpit</h1>
        <nav className="flex flex-col gap-1">
          {navItem("/", "Todos")}
          {navItem("/calendar", "Calendar")}
          {navItem("/standup", "Standup")}
          {navItem("/settings", "Settings")}
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto flex flex-col">
        <Routes>
          <Route path="/" element={<Todos />} />
          <Route path="/calendar" element={<Calendar />} />
          <Route path="/standup" element={<Standup />} />
          <Route path="/trash" element={<Trash />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Todos />} />
        </Routes>
      </main>
    </div>
  );
}
