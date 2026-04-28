import { TrashList } from "../components/TrashList";

export function Trash() {
  return (
    <div className="flex flex-col gap-3 max-w-3xl">
      <h2 className="text-xl font-semibold">Trashcan</h2>
      <p className="text-fgmute text-sm">
        Recently deleted todos. Click Restore to bring one back to the board.
      </p>
      <TrashList />
    </div>
  );
}
