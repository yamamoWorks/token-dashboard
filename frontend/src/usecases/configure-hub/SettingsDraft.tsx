import { createContext, useContext, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { View } from '@bindings/token-monitor-turzx/internal/settings/models';
import { getSettings, useSaveCompactPaging, useSaveSettings } from '../../features/settings/queries';
import { ErrorNotice } from '../../shared/ErrorNotice';
import { publicError } from '../../shared/errors';
import { useDraftDirty } from '../../shared/ExitContext';

type Scope = 'connection' | 'display';
type DisplayChange = { display?: string; profile?: string; style?: string };
type PagingChange = { auto?: boolean; intervalSeconds?: number };
export const automatic = '__automatic__';

function useDraft(saved: View) {
  const save = useSaveSettings();
  const savePaging = useSaveCompactPaging();
  const [source, setSource] = useState(saved.source || 'Local');
  const [url, setURL] = useState(saved.url);
  const [token, setToken] = useState('');
  const [pending, setPending] = useState<DisplayChange>({});
  const [pendingPaging, setPendingPaging] = useState<PagingChange>({});
  const [done, setDone] = useState(false);
  const [failed, setFailed] = useState<Scope | null>(null);
  const connectionDirty = source !== (saved.source || 'Local') || (source === 'Hub' && (url !== saved.url || token !== ''));
  useDraftDirty(connectionDirty);
  const edit = <T,>(set: (value: T) => void) => (value: T) => { set(value); setDone(false); };
  // The Connection page saves with its Save button. The other page's saved values are sent as they are.
  async function submitConnection() {
    setDone(false);
    setFailed(null);
    try {
      const view = await save.mutateAsync({ source, url, token, displayID: saved.displayID, displayProfileID: saved.displayProfileID, limitStyle: saved.limitStyle });
      setSource(view.source || 'Local');
      setURL(view.url);
      setToken('');
      setDone(true);
    } catch { setFailed('connection'); }
  }
  // The Display page applies and saves a choice at once. A failed save leaves the saved value shown.
  async function applyDisplay(change: DisplayChange) {
    setPending(change);
    setFailed(null);
    try {
      await save.mutateAsync({
        source: saved.source || 'Local', url: saved.url, token: '',
        displayID: change.display === undefined ? saved.displayID : change.display === automatic ? '' : change.display,
        displayProfileID: change.profile ?? saved.displayProfileID,
        limitStyle: change.style ?? saved.limitStyle,
      });
    } catch { setFailed('display'); } finally { setPending({}); }
  }
  async function applyPaging(change: PagingChange) {
    const auto = change.auto ?? pendingPaging.auto ?? saved.compactAutoPage;
    const intervalSeconds = change.intervalSeconds ?? pendingPaging.intervalSeconds ?? saved.compactPageIntervalSeconds;
    setPendingPaging(change);
    setFailed(null);
    try {
      await savePaging.mutateAsync({ auto, intervalSeconds });
    } catch { setFailed('display'); } finally { setPendingPaging({}); }
  }
  return {
    saved, source, url, token, connectionDirty, done,
    display: pending.display ?? (saved.displayID || automatic),
    profile: pending.profile ?? saved.displayProfileID,
    style: pending.style ?? (saved.limitStyle || 'Gauges'),
    compactAutoPage: pendingPaging.auto ?? saved.compactAutoPage,
    compactPageIntervalSeconds: pendingPaging.intervalSeconds ?? saved.compactPageIntervalSeconds,
    setSource: edit(setSource), setURL: edit(setURL), setToken: edit(setToken),
    setDisplay: (display: string) => void applyDisplay({ display }),
    setProfile: (profile: string) => void applyDisplay({ profile }),
    setStyle: (style: string) => void applyDisplay({ style }),
    setCompactAutoPage: (auto: boolean) => void applyPaging({ auto }),
    setCompactPageIntervalSeconds: (intervalSeconds: number) => void applyPaging({ intervalSeconds }),
    saving: save.isPending || savePaging.isPending,
    errorFor: (scope: Scope) => failed === scope ? (scope === 'display' ? savePaging.error ?? save.error : save.error) : null,
    fields: (() => {
      const activeError = failed === 'display' ? savePaging.error ?? save.error : save.error;
      return activeError ? publicError(activeError).fieldErrors ?? {} : {};
    })(),
    submitConnection: () => void submitConnection(),
  };
}

type Draft = ReturnType<typeof useDraft>;
const Context = createContext<Draft | null>(null);

function Provider({ saved, children }: { saved: View; children: ReactNode }) {
  return <Context.Provider value={useDraft(saved)}>{children}</Context.Provider>;
}

// The draft spans the Display and Connection pages. Display choices save at once; Connection saves with its button.
export function SettingsDraftProvider({ children }: { children: ReactNode }) {
  const settings = useQuery(getSettings());
  if (settings.data) return <Provider saved={settings.data}>{children}</Provider>;
  return <Context.Provider value={null}><ErrorNotice error={settings.error} />{children}</Context.Provider>;
}

export const useSettingsDraft = () => useContext(Context);
