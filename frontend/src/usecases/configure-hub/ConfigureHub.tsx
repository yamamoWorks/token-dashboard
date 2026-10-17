import type { ReactNode } from 'react';
import { useEffect, useState } from 'react';
import { Alert, Badge, Button, Card, Group, NumberInput, PasswordInput, Select, Stack, Text, TextInput, Title } from '@mantine/core';
import { ErrorNotice } from '../../shared/ErrorNotice';
import { automatic, useSettingsDraft } from './SettingsDraft';
import styles from './ConfigureHub.module.css';

function Row({ title, hint, children }: { title: ReactNode; hint?: string; children: ReactNode }) {
  return <div className={styles.row}>
    <div><Text component="div" fw={600}>{title}</Text>{hint && <Text size="xs" c="dimmed">{hint}</Text>}</div>
    <div>{children}</div>
  </div>;
}

function SaveBar() {
  const draft = useSettingsDraft();
  if (!draft) return null;
  return <>
    <ErrorNotice error={draft.errorFor('connection')} />
    {draft.done && !draft.connectionDirty && <Alert color="green" py="xs" mt="sm">Saved.</Alert>}
    <Group justify="flex-end" mt="md" pt="md" className={styles.bar}>
      <Button loading={draft.saving} onClick={draft.submitConnection}>Save</Button>
    </Group>
  </>;
}

export function ConnectionSettings() {
  const draft = useSettingsDraft();
  if (!draft) return null;
  const { fields, saved } = draft;
  return <Card withBorder padding="md">
    <Title order={4} mb="sm">Usage source</Title>
    <Row title="Data source" hint="Where usage is read from">
      <Select aria-label="Data source" data={['Local', 'Hub']} value={draft.source} allowDeselect={false} error={fields.source}
        onChange={value => { if (value) draft.setSource(value); }} />
    </Row>
    {draft.source === 'Hub' && <>
      <Title order={4} mt="lg" mb="sm">Hub connection</Title>
      <Row title="Hub URL" hint="Where to connect">
        <TextInput aria-label="Hub URL" placeholder="https://hub.example.com" value={draft.url} error={fields.url}
          onChange={e => draft.setURL(e.currentTarget.value)} />
      </Row>
      <Row title={<Group gap="xs" component="span">Access token<Badge size="sm" variant="light" color={saved.tokenSet ? 'green' : 'gray'}>{saved.tokenSet ? 'Set' : 'Not set'}</Badge></Group>}
        hint={saved.tokenSet ? 'Leave blank to keep the saved token.' : undefined}>
        <PasswordInput aria-label="Access token" value={draft.token} error={fields.token} autoComplete="off"
          onChange={e => draft.setToken(e.currentTarget.value)} />
      </Row>
    </>}
    <SaveBar />
  </Card>;
}

// Only the away timeout applies to Rev.A. Keep the former Style and Cycle values
// in the saved settings (SettingsDraft still forwards them) for compatibility.
export function EditSettings() {
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
      <Row title="Away time" hint="No input for this long switches the TURZX off">
        <NumberInput w={110} aria-label="Away time" suffix=" min" min={1} max={240} step={1} allowDecimal={false} clampBehavior="strict" hideControls
          styles={{ input: { textAlign: 'right' } }} error={draft.fields.awayMinutes}
          value={awayText} onChange={setAwayText} onBlur={e => commitAway(e.currentTarget.value)} onKeyDown={e => { if (e.key === 'Enter') commitAway(e.currentTarget.value); }} />
      </Row>
    </Card>
    <ErrorNotice error={draft.errorFor('display')} />
  </Stack>;
}

export function StyleError() {
  const draft = useSettingsDraft();
  return draft ? <ErrorNotice error={draft.errorFor('display')} /> : null;
}

export function DisplaySettings() {
  const draft = useSettingsDraft();
  if (!draft) return null;
  const displays = draft.saved.displays ?? [];
  const options = [
    { value: automatic, label: 'Automatic' },
    ...displays.map(d => ({ value: d.deviceID, label: d.connected ? d.name : `${d.name} (Disconnected)` })),
  ];
  return <Stack gap={4} align="flex-end">
    <Group wrap="nowrap" gap="sm">
      <Title order={5}>Output</Title>
      <Select w={280} aria-label="Output device" data={options} value={draft.display} allowDeselect={false} error={draft.fields.displayID}
        onChange={value => { if (value) draft.setDisplay(value); }} />
    </Group>
    {displays.length === 0 && <Text size="xs" c="dimmed">No TURZX display is connected.</Text>}
  </Stack>;
}
