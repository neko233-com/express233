import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, Link, RouterProvider } from "react-router";
import "@fontsource-variable/geist";
import { ConsoleApp, Home, NotFound } from "./app";
import { ProjectWorkspace } from "./project/workspace";
import "./style.css";

function RouteFailure() {
  return (
    <main className="center-screen">
      <div className="login-card">
        <h1>控制台加载失败</h1>
        <div className="toolbar">
          <button
            className="btn btn-primary"
            type="button"
            onClick={() => window.location.reload()}
          >
            重新加载
          </button>
          <Link className="btn btn-secondary" to="/">
            返回项目工作区
          </Link>
        </div>
      </div>
    </main>
  );
}

const router = createBrowserRouter([
  {
    Component: ConsoleApp,
    ErrorBoundary: RouteFailure,
    children: [
      { index: true, Component: Home },
      {
        path: "dashboard",
        lazy: async () => ({ Component: (await import("./global/dashboard")).Dashboard }),
      },
      { path: "guide", lazy: async () => ({ Component: (await import("./global/guide")).Guide }) },
      {
        path: "agents",
        lazy: async () => ({ Component: (await import("./global/agents")).Agents }),
      },
      {
        path: "server",
        lazy: async () => ({ Component: (await import("./global/settings")).ServerYAML }),
      },
      {
        path: "settings",
        lazy: async () => ({ Component: (await import("./global/settings")).Settings }),
      },
      {
        path: "storage",
        lazy: async () => ({ Component: (await import("./global/storage")).Storage }),
      },
      { path: "projects/:projectId/:tab?", Component: ProjectWorkspace },
      { path: "*", Component: NotFound },
    ],
  },
]);

const root = document.getElementById("root");
if (!root) throw new Error("Console root missing");
createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
