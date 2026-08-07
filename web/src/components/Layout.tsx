type Props = { children: React.ReactNode };

export default function Layout({ children }: Props) {
  return (
    <div className="px-10 py-12 h-screen flex flex-col font-display">
      <nav>
        <h1>crowlr</h1>
      </nav>
      <main className="flex-1">{children}</main>
    </div>
  );
}
