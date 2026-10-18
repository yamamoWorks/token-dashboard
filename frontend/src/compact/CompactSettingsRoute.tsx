import { useEffect, useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { Card, NumberInput, Stack, Text, Title } from '@mantine/core';
import { useSettingsDraft } from '../usecases/configure-hub/SettingsDraft';
import { ErrorNotice } from '../shared/ErrorNotice';
import styles from '../usecases/configure-hub/ConfigureHub.module.css';

function AwaySettings() {
  const draft = useSettingsDraft();
  const [awayText, setAwayText] = useState<number | string>('');
  const minutes = draft?.awayMinutes;
  useEffect(() => { if (minutes !== undefined) setAwayText(minutes); }, [minutes]);
  if (!draft) return null;

  const commitAway = (raw: string) => {
    const value = /\d/.test(raw) ? Number(raw.replace(/\D/g, '')) : NaN;
    if (value >= 1 && value <= 240 && value !== draft.awayMinutes) draft.setAwayMinutes(value);
    else setAwayText(draft.awayMinutes);
  };

  return <Stack gap="md">
    <Card withBorder padding="md">
      <div className={styles.row}>
        <div>
          <Text component="div" fw={600}>Away time</Text>
          <Text size="xs" c="dimmed">No input for this long switches the TURZX off</Text>
        </div>
        <NumberInput w={110} aria-label="Away time" suffix=" min" min={1} max={240} step={1}
          allowDecimal={false} clampBehavior="strict" hideControls
          styles={{ input: { textAlign: 'right' } }} error={draft.fields.awayMinutes}
          value={awayText} onChange={setAwayText}
          onBlur={e => commitAway(e.currentTarget.value)}
          onKeyDown={e => { if (e.key === 'Enter') commitAway(e.currentTarget.value); }} />
      </div>
    </Card>
    <ErrorNotice error={draft.errorFor('display')} />
  </Stack>;
}

export const Route = createFileRoute('/settings')({
  component: () => <Stack gap="md"><Title order={2}>Settings</Title><AwaySettings /></Stack>,
});
