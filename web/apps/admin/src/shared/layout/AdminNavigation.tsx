import { adminRouteSections } from "../../app/routes";
import { adminCopy } from "../i18n";

export function AdminNavigation() {
  return (
    <aside className="rail" aria-label={adminCopy.navigationLabel}>
      <div className="brand">
        <span className="brand-mark">TG</span>
        <span>{adminCopy.brand}</span>
      </div>
      <nav>
        {adminRouteSections.map((item) => (
          <a href={item.href} key={item.id}>
            {item.label}
          </a>
        ))}
      </nav>
    </aside>
  );
}
