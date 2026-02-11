import { useState, useEffect, useRef, useCallback } from "react";

export function useApi(fetchFn, deps = []) {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    const result = await fetchFn();
    if (result?.error) {
      setError(result.error);
      setData(null);
    } else {
      setData(result);
    }
    setLoading(false);
  }, deps);

  useEffect(() => {
    reload();
  }, [reload]);

  return { data, loading, error, reload };
}

export function usePolling(fetchFn, intervalMs = 5000, deps = []) {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const timerRef = useRef(null);

  const poll = useCallback(async () => {
    const result = await fetchFn();
    if (result?.error) {
      setError(result.error);
    } else {
      setData(result);
      setError(null);
    }
  }, deps);

  useEffect(() => {
    poll();
    timerRef.current = setInterval(poll, intervalMs);
    return () => clearInterval(timerRef.current);
  }, [poll, intervalMs]);

  return { data, error, reload: poll };
}
