import { useEffect, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, Group, Image, Text, Title } from '@mantine/core';
import { getPreview, previewKey, subscribePreview } from '../../features/display/queries';
import { ErrorNotice } from '../../shared/ErrorNotice';

// The PNG is a 1920×462 virtual canvas. Rev.A shows its top-left 480×320 pixels.
// Keep the image unmodified: scale the entire canvas with its crop window (4:1).
export function UsagePreview({ title, control, children }: { title: string; control?: ReactNode; children?: ReactNode }) {
  const client = useQueryClient();
  const preview = useQuery(getPreview());
  useEffect(() => subscribePreview(() => void client.invalidateQueries({ queryKey: previewKey })), [client]);
  return <Card withBorder padding="md">
    <Group gap="md" mb="sm"><Title order={4}>{title}</Title>{control}</Group>
    {children}
    <ErrorNotice error={preview.error} />
    {preview.data
      ? <div data-testid="compact-preview" style={{
          width: '100%', maxWidth: 480, aspectRatio: '480 / 320',
          overflow: 'hidden', borderRadius: 'var(--mantine-radius-sm)',
        }}>
          <Image src={preview.data} alt="Display preview"
            style={{ display: 'block', width: '400%', maxWidth: 'none', height: 'auto' }} />
        </div>
      : !preview.error && <Text size="sm" c="dimmed">No image yet.</Text>}
  </Card>;
}
