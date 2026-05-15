import { createRoot } from "react-dom/client";
import { App } from "@/bootstrap/App";
import { AppErrorBoundary } from "@/bootstrap/AppErrorBoundary";
import "@/shared/styles/globals.css";

createRoot(document.getElementById("root")!).render((<AppErrorBoundary><App /></AppErrorBoundary>));
