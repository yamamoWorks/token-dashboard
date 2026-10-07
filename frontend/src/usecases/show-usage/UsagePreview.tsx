import { useEffect, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, Group, Image, Text, Title } from '@mantine/core';
import { getPreview, previewRootKey, subscribePreview } from '../../features/display/queries';
import { ErrorNotice } from '../../shared/ErrorNotice';
import { useSettingsDraft } from '../configure-hub/SettingsDraft';
import { previewAspectRatio, previewWidth } from './preview';
import styles from './UsagePreview.module.css';

// Shows the latest data for the selected preview page. Compact displays keep the page fixed until
// the user clicks a page dot; TURZX automatic paging is independent from this window state.
export function UsagePreview({ title, control, children }: { title: string; control?: ReactNode; children?: ReactNode }) {
  const client = useQueryClient();
  const draft = useSettingsDraft();
  const [page, setPage] = useState(0);
  const preview = useQuery(getPreview(page));
  const profile = draft?.profile;
  const aspectRatio = previewAspectRatio(draft?.saved.displayProfiles ?? [], profile);
  const width = previewWidth(draft?.saved.displayProfiles ?? [], profile);

  useEffect(() => subscribePreview(() => void client.invalidateQueries({ queryKey: previewRootKey })), [client]);
  useEffect(() => setPage(0), [profile]);
  useEffect(() => {
    if (preview.data && preview.data.page !== page) setPage(preview.data.page);
  }, [page, preview.data]);

  return <Card withBorder padding="md">
    <Group gap="md" mb="sm"><Title order={4}>{title}</Title>{control}</Group>
    {children}
    <ErrorNotice error={preview.error} />
    {preview.data?.image
      ? <div className={styles.preview} style={{ width }}>
          <Image src={preview.data.image} alt="Display preview" radius="sm" w="100%" style={{ aspectRatio }} />
          {preview.data.pageCount > 1 && <div className={styles.pages} aria-label="Preview pages">
            {Array.from({ length: preview.data.pageCount }, (_, index) =>
              <button key={index} type="button" className={styles.page}
                aria-label={`Preview page ${index + 1}`} aria-current={index === preview.data.page ? 'page' : undefined}
                onClick={() => setPage(index)}>
                {index === preview.data.page ? '●' : '○'}
              </button>)}
          </div>}
        </div>
      : !preview.error && <Text size="sm" c="dimmed">No image yet.</Text>}
  </Card>;
}
