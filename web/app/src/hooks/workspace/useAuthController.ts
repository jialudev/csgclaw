import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { beginAuthLogin, fetchAuthStatus, logoutAuth } from "@/api/auth";
import { errorMessage } from "@/api/client";
import { emptyAuthStatus, isAuthenticated, normalizeAuthStatus, normalizeLoginResponse } from "@/models/auth";
import type { AuthStatus } from "@/models/auth";
import type { TranslateFn } from "@/models/conversations";
import { workspaceQueryKeys } from "./workspaceQueries";

const LOGIN_POLL_INTERVAL_MS = 2000;
const LOGIN_POLL_TIMEOUT_MS = 120000;

export type AuthController = {
  busy: boolean;
  error: string;
  login: () => Promise<void>;
  logout: () => Promise<void>;
  pending: boolean;
  status: AuthStatus;
};

export function useAuthController(t: TranslateFn): AuthController {
  const queryClient = useQueryClient();
  const [busyAction, setBusyAction] = useState<"login" | "logout" | "">("");
  const [authError, setAuthError] = useState("");
  const [loginPending, setLoginPending] = useState(false);

  const statusQuery = useQuery({
    queryKey: workspaceQueryKeys.authStatus(),
    queryFn: fetchNormalizedAuthStatus,
    retry: 0,
  });

  const status = useMemo(() => statusQuery.data ?? emptyAuthStatus(), [statusQuery.data]);

  const setStatus = useCallback(
    (next: AuthStatus) => {
      queryClient.setQueryData(workspaceQueryKeys.authStatus(), next);
    },
    [queryClient],
  );

  const refreshStatus = useCallback(async () => {
    return queryClient.fetchQuery({
      queryKey: workspaceQueryKeys.authStatus(),
      queryFn: fetchNormalizedAuthStatus,
      retry: 0,
    });
  }, [queryClient]);

  const login = useCallback(async () => {
    if (busyAction) {
      return;
    }
    setBusyAction("login");
    setAuthError("");
    try {
      const loginResp = normalizeLoginResponse(await beginAuthLogin(window.location.href));
      if (!loginResp.login_url) {
        throw new Error(t("csghubLoginURLMissing"));
      }
      setLoginPending(true);
      window.location.assign(loginResp.login_url);
    } catch (err) {
      setLoginPending(false);
      setAuthError(errorMessage(err, t("csghubLoginFailed")));
    } finally {
      setBusyAction("");
    }
  }, [busyAction, t]);

  const logout = useCallback(async () => {
    if (busyAction) {
      return;
    }
    setBusyAction("logout");
    setAuthError("");
    try {
      const next = normalizeAuthStatus(await logoutAuth());
      setStatus(next);
      setLoginPending(false);
    } catch (err) {
      setAuthError(errorMessage(err, t("csghubLogoutFailed")));
    } finally {
      setBusyAction("");
    }
  }, [busyAction, setStatus, t]);

  useEffect(() => {
    if (!loginPending) {
      return undefined;
    }
    const startedAt = Date.now();
    let cancelled = false;

    async function pollStatus() {
      try {
        const next = await refreshStatus();
        if (cancelled) {
          return;
        }
        if (isAuthenticated(next)) {
          setLoginPending(false);
          setAuthError("");
          return;
        }
      } catch (_) {
        // Keep polling until timeout; transient callback timing is expected.
      }
      if (!cancelled && Date.now() - startedAt >= LOGIN_POLL_TIMEOUT_MS) {
        setLoginPending(false);
        setAuthError(t("csghubLoginTimedOut"));
      }
    }

    const firstPoll = window.setTimeout(pollStatus, 1000);
    const interval = window.setInterval(pollStatus, LOGIN_POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearTimeout(firstPoll);
      window.clearInterval(interval);
    };
  }, [loginPending, refreshStatus, t]);

  return {
    busy: Boolean(busyAction),
    error: authError || (statusQuery.isError ? errorMessage(statusQuery.error, t("csghubStatusFailed")) : ""),
    login,
    logout,
    pending: loginPending,
    status,
  };
}

async function fetchNormalizedAuthStatus(): Promise<AuthStatus> {
  return normalizeAuthStatus(await fetchAuthStatus());
}
