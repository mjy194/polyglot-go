import { useCallback, useEffect, useState } from 'react';
import { message } from 'antd';

interface FetchState<T> {
  data: T | undefined;
  loading: boolean;
  error: string | undefined;
  reload: () => void;
}

// 通用数据加载 hook。deps 变化时自动重拉;reload() 手动刷新。
export function useFetch<T>(fn: () => Promise<T>, deps: unknown[] = []): FetchState<T> {
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [tick, setTick] = useState(0);

  const reload = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    fn()
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((err) => {
        if (cancelled) return;
        const msg = err?.response?.data?.error || err?.message || '请求失败';
        setError(msg);
        message.error(msg);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  return { data, loading, error, reload };
}
