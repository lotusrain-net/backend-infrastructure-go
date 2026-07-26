import { create } from "zustand";
import type { UserProfile } from "@/types/api";

interface AuthState {
  user: UserProfile | null;
  setCurrentUser: (user: UserProfile | null) => void;
  clearAuthState: () => void;
}

const defaultState = {
  user: null as UserProfile | null,
};

export const useAuthStore = create<AuthState>((set) => ({
  ...defaultState,
  setCurrentUser: (user) => set({ user }),
  clearAuthState: () => set({ user: null }),
}));

export function getAuthState() {
  return useAuthStore.getState();
}

export function setCurrentUser(user: UserProfile | null) {
  useAuthStore.getState().setCurrentUser(user);
}

export function clearAuthState() {
  useAuthStore.getState().clearAuthState();
}

export function resetAuthStore() {
  useAuthStore.setState(defaultState);
}
