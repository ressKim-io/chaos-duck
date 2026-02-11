import { useState, useEffect, useRef, useCallback } from "react";
import { getExperiment } from "./api";

/**
 * useSSE - Connect to SSE stream for real-time experiment updates.
 * Falls back to polling if EventSource connection fails.
 */
export default function useSSE(experimentId) {
  const [data, setData] = useState(null);
  const [connected, setConnected] = useState(false);
  const [done, setDone] = useState(false);
  const esRef = useRef(null);
  const pollRef = useRef(null);
  const fallbackRef = useRef(false);

  const startPolling = useCallback(() => {
    if (fallbackRef.current || done) return;
    fallbackRef.current = true;
    setConnected(false);
    const poll = async () => {
      const res = await getExperiment(experimentId);
      if (!res?.error) {
        setData(res);
        const terminal = ["completed", "failed", "rolled_back", "emergency_stopped"];
        if (terminal.includes(res.status)) {
          setDone(true);
          clearInterval(pollRef.current);
        }
      }
    };
    poll();
    pollRef.current = setInterval(poll, 2000);
  }, [experimentId, done]);

  useEffect(() => {
    if (!experimentId) return;

    fallbackRef.current = false;
    setDone(false);
    setData(null);
    setConnected(false);

    try {
      const es = new EventSource(`/api/chaos/experiments/${experimentId}/stream`);
      esRef.current = es;

      es.addEventListener("experiment", (e) => {
        try {
          const parsed = JSON.parse(e.data);
          setData(parsed);
          setConnected(true);
        } catch { /* ignore parse errors */ }
      });

      es.addEventListener("done", () => {
        setDone(true);
        es.close();
      });

      es.onerror = () => {
        es.close();
        startPolling();
      };
    } catch {
      startPolling();
    }

    return () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [experimentId, startPolling]);

  return { data, connected, done };
}
