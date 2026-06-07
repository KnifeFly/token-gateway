import { Button } from "@token-gateway/ui";
import type { FormEvent } from "react";

interface AdminLoginViewProps {
  busy: boolean;
  message: string;
  onLogin: (email: string, password: string) => Promise<void>;
}

export function AdminLoginView({ busy, message, onLogin }: AdminLoginViewProps) {
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    await onLogin(String(data.get("email") ?? ""), String(data.get("password") ?? ""));
  }

  return (
    <main className="login-shell">
      <form className="login-panel" onSubmit={handleSubmit}>
        <div>
          <span className="brand-mark">TG</span>
          <h1>管理端登录</h1>
          <p>使用操作员账号进入 Admin Web BFF，浏览器不会持有 control token。</p>
        </div>

        <label>
          邮箱
          <input name="email" autoComplete="username" defaultValue="admin@example.com" />
        </label>

        <label>
          密码
          <input
            name="password"
            autoComplete="current-password"
            defaultValue="admin-local"
            type="password"
          />
        </label>

        {message ? <p className="form-message">{message}</p> : null}

        <Button disabled={busy} type="submit" variant="primary">
          {busy ? "登录中" : "登录"}
        </Button>
      </form>
    </main>
  );
}
