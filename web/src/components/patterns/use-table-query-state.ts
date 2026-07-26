"use client";

import * as React from "react";

export interface TableQueryState {
  page: number;
  size: number;
  search: string;
}

const defaultState: TableQueryState = {
  page: 1,
  size: 10,
  search: "",
};

function parsePositiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

export function createTableQueryState(searchParams: URLSearchParams): TableQueryState {
  return {
    page: parsePositiveInteger(searchParams.get("page"), defaultState.page),
    size: parsePositiveInteger(searchParams.get("size"), defaultState.size),
    search: searchParams.get("search") ?? defaultState.search,
  };
}

export function useTableQueryState(initialSearchParams = new URLSearchParams()) {
  const [state, setState] = React.useState<TableQueryState>(() =>
    createTableQueryState(initialSearchParams),
  );

  const setPage = React.useCallback((page: number) => {
    setState((current) => ({ ...current, page }));
  }, []);

  const setSize = React.useCallback((size: number) => {
    setState((current) => ({ ...current, size, page: 1 }));
  }, []);

  const setSearch = React.useCallback((search: string) => {
    setState((current) => ({ ...current, search, page: 1 }));
  }, []);

  const reset = React.useCallback(() => {
    setState(defaultState);
  }, []);

  return {
    state,
    setPage,
    setSize,
    setSearch,
    reset,
  };
}
