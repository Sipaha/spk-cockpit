import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import { Todos } from "./pages/Todos";
import { Popover } from "./pages/Popover";
import { Calendar } from "./pages/Calendar";
import { Settings } from "./pages/Settings";
import { Standup } from "./pages/Standup";
import { MeetingPopup } from "./pages/MeetingPopup";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/popover" element={<Popover />} />
        <Route path="/popup-meeting" element={<MeetingPopup />} />
        <Route path="*" element={<MainShell />} />
      </Routes>
    </BrowserRouter>
  );
}

function MainShell() {
  const loc = useLocation();
  const navItem = (to: string, label: string) => (
    <Link to={to} className={loc.pathname === to ? "text-fg" : "text-fgmute"}>
      {label}
    </Link>
  );
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">spk-cockpit</h1>
        <nav className="flex flex-col gap-1">
          {navItem("/", "Todos")}
          {navItem("/calendar", "Calendar")}
          {navItem("/standup", "Standup")}
          {navItem("/settings", "Settings")}
          {navItem("/popover", "Compact view")}
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Routes>
          <Route path="/" element={<Todos />} />
          <Route path="/calendar" element={<Calendar />} />
          <Route path="/standup" element={<Standup />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Todos />} />
        </Routes>
      </main>
    </div>
  );
}
