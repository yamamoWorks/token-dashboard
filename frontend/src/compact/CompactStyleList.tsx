import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Badge, Card, Group, Image, Stack, Text } from '@mantine/core';
import { renderStylePreview, subscribeThemeData } from '../features/display/theme-renderer';
import { loadStyleCatalog, type StyleDefinition } from '../features/styles/catalog';
import { DeleteStyle } from '../usecases/delete-style/DeleteStyle';
import { ErrorNotice } from '../shared/ErrorNotice';
import { CompactPreviewViewport } from './CompactUsagePreview';

// The Rev.A display always renders Compact, irrespective of the saved legacy style value.
// Built-in IDs are never exposed to the external-style Delete action.
const builtins = [
  { id: 'gauges', name: 'Gauges', builtin: true },
  { id: 'bars', name: 'Bars', builtin: true },
  { id: 'compact', name: 'Compact 3.5-inch', builtin: true },
];
const builtinIds = new Set(builtins.map(style => style.id));

function styleCards(catalog: StyleDefinition[]) {
  const external = catalog.filter(style => !builtinIds.has(style.id))
    .sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
    .map(style => ({ ...style, builtin: false }));
  return [...builtins, ...external];
}

export function StyleList() {
  const catalog = useQuery({ queryKey: ['styles', 'catalog'], queryFn: loadStyleCatalog });
  const cards = useMemo(() => styleCards(catalog.data ?? []), [catalog.data]);
  const [revision, setRevision] = useState(0);
  const [images, setImages] = useState<Record<string, string>>({});
  const [failures, setFailures] = useState<Record<string, string>>({});

  useEffect(() => subscribeThemeData(() => setRevision(value => value + 1)), []);
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const next: Record<string, string> = {};
      const errors: Record<string, string> = {};
      for (const card of cards) {
        try {
          next[card.id] = await renderStylePreview(card.id);
        } catch (error) {
          errors[card.id] = error instanceof Error ? error.message : String(error);
        }
        if (cancelled) return;
      }
      if (!cancelled) {
        setImages(next);
        setFailures(errors);
      }
    })();
    return () => { cancelled = true; };
  }, [cards, revision]);

  return <Stack gap="md">
    <ErrorNotice error={catalog.error} />
    <Text size="sm" c="dimmed">Compact 3.5-inch is the fixed display style for Rev.A.</Text>
    {cards.map(card => <Card key={card.id} component="article" aria-label={card.name} withBorder padding="md">
      <Group gap="sm" mb="sm" justify="space-between">
        <Group gap="sm">
          <Text fw={700}>{card.name}</Text>
          {card.builtin && <Badge variant="light" color="gray" tt="none">Built-in</Badge>}
          {card.id === 'compact' && <Badge variant="light" tt="none">In use</Badge>}
        </Group>
        {!card.builtin && <DeleteStyle style={card} />}
      </Group>
      {images[card.id]
        ? card.id === 'compact'
          ? <CompactPreviewViewport src={images[card.id]} alt={`${card.name} preview`} />
          : <Image src={images[card.id]} alt={`${card.name} preview`} radius="sm" style={{ aspectRatio: '1920 / 462' }} />
        : <Text size="sm" c={failures[card.id] ? 'red' : 'dimmed'}>
            {failures[card.id] ?? 'No image yet.'}
          </Text>}
    </Card>)}
  </Stack>;
}
