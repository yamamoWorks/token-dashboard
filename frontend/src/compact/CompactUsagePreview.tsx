import { useEffect, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, Group, Image, Text, Title } from '@mantine/core';
import { getPreview, previewKey, subscribePreview } from '../features/display/queries';
import { ErrorNotice } from '../shared/ErrorNotice';

// Display.Preview() is already a 1920x462 PNG. CSS clips its top-left 480x320
// without changing the source image or invoking the renderer a second time.
export function CompactPreviewViewport({ src, alt }: { src: string; alt: string }) {
  return <div data-testid="compact-preview" style={{
    width: '100%', maxWidth: 480, aspectRatio: '480 / 320',
    overflow: 'hidden', borderRadius: 'var(--mantine-radius-sm)',
  }}>
    <Image src={src} alt={alt} style={{
      display: 'block', width: '400%', maxWidth: 'none', height: 'auto',
    }} />
  </div>;
}

export function CompactUsagePreview({ title, children }: { title: string; children?: ReactNode }) {
  const client = useQueryClient();
  const preview = useQuery(getPreview());
  useEffect(() => subscribePreview(() => void client.invalidateQueries({ queryKey: previewKey })), [client]);
  return <Card withBorder padding="md">
    <Group gap="md" mb="sm"><Title order={4}>{title}</Title></Group>
    {children}
    <ErrorNotice error={preview.error} />
    {preview.data
      ? <CompactPreviewViewport src={preview.data} alt="Display preview" />
      : !preview.error && <Text size="sm" c="dimmed">No image yet.</Text>}
  </Card>;
}
