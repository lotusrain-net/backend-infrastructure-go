"use client";

import * as React from "react";
import { createAppearancePreferences } from "@lotusrain-net/backend-infrastructure-web/theme";
import {
  useCurrentUserPreferencesQuery,
  usePutCurrentUserPreferencesMutation,
} from "@/features/preferences/api";
import {
  getThemeState,
  type PreferenceSyncStatus,
  useThemeStore,
} from "@/stores/theme-store";
import type { AccountPreferences } from "@/types/api";

interface AppearancePreferencesContextValue {
  preferences: AccountPreferences;
  syncStatus: PreferenceSyncStatus;
  syncError: string | null;
  previewPreferences: (changes: Partial<AccountPreferences>) => void;
  updatePreferences: (changes: Partial<AccountPreferences>) => void;
  retry: () => void;
}

const AppearancePreferencesContext = React.createContext<AppearancePreferencesContextValue | null>(null);

export function AppearancePreferencesProvider({
  userID,
  children,
}: {
  userID: string;
  children: React.ReactNode;
}) {
  const preferences = useThemeStore((state) => state.preferences);
  const syncStatus = useThemeStore((state) => state.syncStatus);
  const syncError = useThemeStore((state) => state.syncError);
  const failureKind = useThemeStore((state) => state.failureKind);
  const pendingPreferences = useThemeStore((state) => state.pendingPreferences);
  const switchUser = useThemeStore((state) => state.switchUser);
  const previewLocal = useThemeStore((state) => state.previewLocal);
  const preview = useThemeStore((state) => state.preview);
  const applyRemote = useThemeStore((state) => state.applyRemote);
  const markSyncFailed = useThemeStore((state) => state.markSyncFailed);
  const markSyncing = useThemeStore((state) => state.markSyncing);
  const query = useCurrentUserPreferencesQuery(userID);
  const mutation = usePutCurrentUserPreferencesMutation();
  const queuedPreferences = React.useRef<AccountPreferences | null>(null);
  const syncInFlight = React.useRef(false);
  const flushQueueRef = React.useRef<() => void>(() => undefined);

  React.useEffect(() => {
    // Do not send an unsent payload belonging to the previous authenticated
    // account if this provider receives a different user ID.
    queuedPreferences.current = null;
    switchUser(userID);
  }, [switchUser, userID]);

  React.useEffect(() => {
    const state = getThemeState();
    if (query.data && !state.pendingPreferences && !state.isPreviewing) {
      applyRemote(userID, query.data);
    }
  }, [applyRemote, query.data, userID]);

  React.useEffect(() => {
    if (query.isError) {
      markSyncFailed(userID, query.error, "load");
    }
  }, [markSyncFailed, query.error, query.isError, userID]);

  const flushQueue = React.useCallback(() => {
    if (syncInFlight.current || !queuedPreferences.current) {
      return;
    }

    const requestedPreferences = queuedPreferences.current;
    queuedPreferences.current = null;
    syncInFlight.current = true;

    void mutation
      .mutateAsync(requestedPreferences)
      .then((serverPreferences) => {
        // A newer preview is queued while this request is in flight. Do not let
        // this older response repaint the interface before the newer PUT runs.
        const state = getThemeState();
        if (!queuedPreferences.current && !state.isPreviewing && state.userID === userID) {
          applyRemote(userID, serverPreferences);
        }
      })
      .catch((error: unknown) => {
        // An obsolete request should not surface an error while a newer complete
        // preference payload is already queued for synchronization.
        const state = getThemeState();
        if (!queuedPreferences.current && !state.isPreviewing && state.userID === userID) {
          markSyncFailed(userID, error, "save");
        }
      })
      .finally(() => {
        syncInFlight.current = false;
        if (queuedPreferences.current) {
          flushQueueRef.current();
        }
      });
  }, [applyRemote, markSyncFailed, mutation, userID]);

  React.useEffect(() => {
    flushQueueRef.current = flushQueue;
  }, [flushQueue]);

  const previewPreferences = React.useCallback(
    (changes: Partial<AccountPreferences>) => {
      const state = getThemeState();
      if (state.userID !== userID) {
        return;
      }

      previewLocal(userID, createAppearancePreferences({ ...state.preferences, ...changes }));
    },
    [previewLocal, userID],
  );

  const updatePreferences = React.useCallback(
    (changes: Partial<AccountPreferences>) => {
      const state = getThemeState();
      if (state.userID !== userID) {
        return;
      }

      const nextPreferences = createAppearancePreferences({ ...state.preferences, ...changes });
      preview(userID, nextPreferences);
      queuedPreferences.current = nextPreferences;
      flushQueueRef.current();
    },
    [preview, userID],
  );

  const retry = React.useCallback(() => {
    if (failureKind === "load") {
      switchUser(userID);
      void query.refetch();
      return;
    }

    if (pendingPreferences) {
      markSyncing(userID);
      queuedPreferences.current = pendingPreferences;
      flushQueueRef.current();
    }
  }, [failureKind, markSyncing, pendingPreferences, query, switchUser, userID]);

  const value = React.useMemo(
    () => ({ preferences, syncStatus, syncError, previewPreferences, updatePreferences, retry }),
    [preferences, previewPreferences, retry, syncError, syncStatus, updatePreferences],
  );

  return <AppearancePreferencesContext.Provider value={value}>{children}</AppearancePreferencesContext.Provider>;
}

export function useAppearancePreferences() {
  const context = React.useContext(AppearancePreferencesContext);
  if (!context) {
    throw new Error("Appearance controls require an authenticated preference provider.");
  }
  return context;
}
