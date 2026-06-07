import { adminRouteSections } from "../../app/routes";
import { adminCopy } from "../i18n";

interface AdminNavigationProps {
  activeRouteID: string;
  onNavigate: (routeID: string) => void;
}

export function AdminNavigation({ activeRouteID, onNavigate }: AdminNavigationProps) {
  return (
    <aside className="rail" aria-label={adminCopy.navigationLabel}>
      <div className="brand">
        <span className="brand-mark">TG</span>
        <span>{adminCopy.brand}</span>
      </div>
      <nav>
        {adminRouteSections.map((item) => (
          <a
            aria-current={activeRouteID === item.id ? "page" : undefined}
            href={item.routePrefix}
            key={item.id}
            onClick={(event) => {
              event.preventDefault();
              onNavigate(item.id);
            }}
          >
            {item.label}
          </a>
        ))}
      </nav>
    </aside>
  );
}
