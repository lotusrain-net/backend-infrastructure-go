"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

interface TabsContextValue {
  value: string;
  setValue: (value: string) => void;
  idPrefix: string;
}

const TabsContext = React.createContext<TabsContextValue | null>(null);

function useTabsContext() {
  const context = React.useContext(TabsContext);
  if (!context) {
    throw new Error("Tab components must be rendered inside Tabs.");
  }
  return context;
}

export function Tabs({
  defaultValue,
  value: controlledValue,
  onValueChange,
  className,
  children,
}: {
  defaultValue: string;
  value?: string;
  onValueChange?: (value: string) => void;
  className?: string;
  children: React.ReactNode;
}) {
  const [uncontrolledValue, setUncontrolledValue] = React.useState(defaultValue);
  const idPrefix = React.useId();
  const value = controlledValue ?? uncontrolledValue;
  const setValue = React.useCallback(
    (nextValue: string) => {
      if (controlledValue === undefined) {
        setUncontrolledValue(nextValue);
      }
      onValueChange?.(nextValue);
    },
    [controlledValue, onValueChange],
  );

  return (
    <TabsContext.Provider value={{ value, setValue, idPrefix }}>
      <div className={className}>{children}</div>
    </TabsContext.Provider>
  );
}

export function TabList({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("flex w-fit border-b border-[color:var(--border-subtle)]", className)}
      role="tablist"
      {...props}
    />
  );
}

export function TabTrigger({
  value,
  className,
  onKeyDown,
  ...props
}: Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "value"> & { value: string }) {
  const context = useTabsContext();
  const selected = context.value === value;
  const id = `${context.idPrefix}-tab-${value}`;
  const panelID = `${context.idPrefix}-panel-${value}`;

  function handleKeyDown(event: React.KeyboardEvent<HTMLButtonElement>) {
    onKeyDown?.(event);
    if (event.defaultPrevented || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) {
      return;
    }

    const tabList = event.currentTarget.closest('[role="tablist"]');
    const tabs = tabList ? Array.from(tabList.querySelectorAll<HTMLButtonElement>('[role="tab"]')) : [];
    const currentIndex = tabs.indexOf(event.currentTarget);
    if (currentIndex < 0 || tabs.length === 0) {
      return;
    }

    event.preventDefault();
    const nextIndex =
      event.key === "Home"
        ? 0
        : event.key === "End"
          ? tabs.length - 1
          : (currentIndex + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length;
    const next = tabs[nextIndex];
    const nextValue = next?.dataset.value;
    if (next && nextValue) {
      context.setValue(nextValue);
      next.focus();
    }
  }

  return (
    <button
      type="button"
      id={id}
      data-value={value}
      role="tab"
      aria-selected={selected}
      aria-controls={panelID}
      tabIndex={selected ? 0 : -1}
      className={cn(
        "border-b-2 px-3 py-2 text-[length:var(--text-label)] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)]",
        selected
          ? "border-[color:var(--accent-primary)] text-[color:var(--accent-primary)]"
          : "border-transparent text-[color:var(--fg-muted)] hover:text-[color:var(--fg-default)]",
        className,
      )}
      onClick={() => context.setValue(value)}
      onKeyDown={handleKeyDown}
      {...props}
    />
  );
}

export function TabPanel({
  value,
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & { value: string }) {
  const context = useTabsContext();
  if (context.value !== value) {
    return null;
  }

  return (
    <div
      id={`${context.idPrefix}-panel-${value}`}
      role="tabpanel"
      aria-labelledby={`${context.idPrefix}-tab-${value}`}
      className={className}
      {...props}
    >
      {children}
    </div>
  );
}
