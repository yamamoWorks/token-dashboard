import { createFileRoute } from '@tanstack/react-router';
import { Group, Stack, Title } from '@mantine/core';
import { DisplaySettings, StyleError } from '../usecases/configure-hub/ConfigureHub';
import { UsageLimitSelect } from '../usecases/show-usage/UsageLimitSelect';
import { CompactUsagePreview } from './CompactUsagePreview';

export const Route = createFileRoute('/')({
  component: () => <Stack gap="md" style={{ flex: 1, minHeight: 0 }}>
    <Group justify="space-between" align="flex-start">
      <Title order={2}>Display settings</Title>
      <DisplaySettings />
    </Group>
    <CompactUsagePreview title="Preview"><StyleError /></CompactUsagePreview>
    <UsageLimitSelect />
  </Stack>,
});
