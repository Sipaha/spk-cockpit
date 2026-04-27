export function TagPill({ name }: { name: string }) {
  return (
    <span className="inline-block px-2 py-0.5 rounded-full bg-bgmute text-fgmute text-xs">
      #{name}
    </span>
  );
}
