import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';
import * as Display from '@bindings/token-monitor-turzx/internal/display/service';

export const previewRootKey = ['display', 'preview'] as const;
export const previewKey = (page: number) => [...previewRootKey, page] as const;
export const getPreview = (page: number) => queryOptions({ queryKey: previewKey(page), queryFn: () => Display.PreviewPage(page) });
export const subscribePreview = (handler: () => void) => Events.On('display:updated', handler);

export const limitsKey = ['display', 'limits'] as const;
export const getLimits = () => queryOptions({ queryKey: limitsKey, queryFn: () => Display.Limits() });
export function useSetShown() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ keys, shown }: { keys: string[]; shown: boolean }) => Display.SetShown(keys, shown),
    onSuccess: (contracts) => { client.setQueryData(limitsKey, contracts); },
  });
}
