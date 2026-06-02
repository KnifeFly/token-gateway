import { navItems } from "../../app/routes";

export function AdminNavigation() {
  return (
    <aside className="rail" aria-label="Admin navigation">
      <div className="brand">
        <span className="brand-mark">TG</span>
        <span>Admin</span>
      </div>
      <nav>
        {navItems.map((item) => (
          <a href={`#${item.toLowerCase()}`} key={item}>
            {item}
          </a>
        ))}
      </nav>
    </aside>
  );
}
