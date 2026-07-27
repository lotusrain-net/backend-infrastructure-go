"use client";

import * as React from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

export interface TableQueryState {
  page: number;
  size: number;
  search?: string;
}

export interface TableQueryCodec<T extends TableQueryState> {
  defaultState: T;
  keys: readonly string[];
  parse: (searchParams: URLSearchParams) => T;
  serialize: (state: T) => Record<string, string | undefined>;
  resetPageOnChangeKeys?: ReadonlyArray<keyof T>;
}

export type TableQueryUpdate<T> = Partial<T> | ((current: T) => Partial<T>);

function asSearchParams(searchParams: { toString(): string }) {
  return new URLSearchParams(searchParams.toString());
}

function hasChanged<T extends TableQueryState>(current: T, next: T, keys: ReadonlyArray<keyof T>) {
  return keys.some((key) => current[key] !== next[key]);
}

export function useTableQueryState<T extends TableQueryState>(codec: TableQueryCodec<T>) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const search = searchParams.toString();
  const state = React.useMemo(() => codec.parse(new URLSearchParams(search)), [codec, search]);

  const replaceState = React.useCallback((next: T) => {
    const nextParams = asSearchParams(searchParams);
    for (const key of codec.keys) {
      nextParams.delete(key);
    }
    for (const [key, value] of Object.entries(codec.serialize(next))) {
      if (value !== undefined && value !== "") {
        nextParams.set(key, value);
      }
    }
    const query = nextParams.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
  }, [codec, pathname, router, searchParams]);

  const setState = React.useCallback((update: TableQueryUpdate<T>) => {
    const current = codec.parse(asSearchParams(searchParams));
    const patch = typeof update === "function" ? update(current) : update;
    const next = { ...current, ...patch } as T;
    if (codec.resetPageOnChangeKeys && hasChanged(current, next, codec.resetPageOnChangeKeys)) {
      next.page = codec.defaultState.page;
    }
    replaceState(next);
  }, [codec, replaceState, searchParams]);

  const reset = React.useCallback(() => {
    replaceState(codec.defaultState);
  }, [codec.defaultState, replaceState]);

  return { state, setState, reset };
}
