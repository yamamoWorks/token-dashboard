import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import Handlebars from 'handlebars';
import { describe, expect, it } from 'vitest';
import { readThemeManifests } from '../../build/theme-build';

const themesRoot = join(import.meta.dirname, '../../../themes');
const template = Handlebars.compile(
  readFileSync(join(themesRoot, 'compact/template.hbs'), 'utf8'),
  { strict: true },
);
const stylesheet = readFileSync(join(themesRoot, 'compact/style.css'), 'utf8');

type Window = {
  id: string; label: string; durationLabel: string; resetText: string;
  remainingPercent: number | null; hasRemaining: boolean; tone: string; percentText: string;
};
type Circle = { name: string; windows: Window[] };

function window(id: string, label: string, remaining: number | null, duration = '5h'): Window {
  return {
    id, label, durationLabel: duration, resetText: '2h 13m', remainingPercent: remaining,
    hasRemaining: remaining !== null,
    tone: remaining === null || remaining >= 40 ? 'normal' : 'danger',
    percentText: remaining === null ? '—' : `${remaining}%`,
  };
}
function panel(provider: string, circles: Circle[]) {
  return { provider, plan: 'Pro', icon: '/assets/agents/claude.png', span: circles.length, circles };
}
function data(panels: ReturnType<typeof panel>[], available = true, waitingMessage = 'Waiting for Hub') {
  return {
    available, waitingMessage,
    tokens: [
      { label: 'Today', tokensText: '12,345,678', costText: '$48.32', tokensFontSize: 42 },
      { label: 'Month', tokensText: '99,999', costText: '$1', tokensFontSize: 42 },
    ],
    gaugePanels: panels,
    barColumns: [],
  };
}
function render(value: ReturnType<typeof data>) {
  return new DOMParser().parseFromString(template(value), 'text/html');
}

describe('compact theme', () => {
  it('is picked up automatically by the existing 1920x462 theme builder', () => {
    const compact = readThemeManifests(themesRoot).find(({ manifest }) => manifest.id === 'compact');
    expect(compact?.manifest).toMatchObject({
      formatVersion: 1, id: 'compact', width: 1920, height: 462,
      template: 'template.hbs', stylesheet: 'style.css',
    });
    expect(stylesheet).toContain('width: 480px;');
    expect(stylesheet).toContain('height: 320px;');
  });

  it.each([
    ['same circle', (a: Window, b: Window) => [{ name: '', windows: [a, b] }]],
    ['different circles', (a: Window, b: Window) => [
      { name: 'Gemini', windows: [a] }, { name: 'Claude', windows: [b] },
    ]],
  ] as const)('flattens two windows into one SVG when %s', (_description, groups) => {
    const a = window('session', 'Session', 35);
    const b = window('weekly', 'Claude Weekly', 82, '7d');
    const document = render(data([panel('antigravity', groups(a, b))]));
    const rings = [...document.querySelectorAll('.meter > .window-ring')];
    expect(rings).toHaveLength(2);
    expect(rings.map(item => item.getAttribute('data-window-id'))).toEqual(['session', 'weekly']);
    expect(document.querySelectorAll('.meter')).toHaveLength(1);
    expect([...document.querySelectorAll('.meter-value strong')].map(item => item.textContent)).toEqual(['35%', '82%']);
    expect([...document.querySelectorAll('.original-label')].map(item => item.textContent)).toEqual(['Session', 'Claude Weekly']);
    expect([...document.querySelectorAll('.reset-time')].map(item => item.textContent?.trim())).toEqual(['2h 13m', '2h 13m']);
    expect(rings[0].querySelector('.ring-outer.ring-fill')?.getAttribute('stroke-dasharray')).toBe('35 100');
    expect(rings[1].querySelector('.ring-inner.ring-fill')?.getAttribute('stroke-dasharray')).toBe('82 100');
    expect(document.querySelector('.token-count')?.textContent).toBe('12,345,678');
    expect(document.querySelector('.token-cost')?.textContent).toBe('$48.32');
    expect(document.querySelectorAll('.token-count')).toHaveLength(1);
  });

  it('renders one provider/window and two providers without any page controls', () => {
    const one = render(data([panel('claude', [{ name: '', windows: [window('daily', 'Daily', 50)] }])]));
    expect(one.querySelectorAll('.provider-card')).toHaveLength(1);
    expect(one.querySelectorAll('.window-ring')).toHaveLength(1);
    const two = render(data([
      panel('claude', [{ name: '', windows: [window('daily', 'Daily', 50)] }]),
      panel('codex', [{ name: '', windows: [window('weekly', 'Weekly', 10, '7d')] }]),
    ]));
    expect([...two.querySelectorAll('.provider-name')].map(item => item.textContent)).toEqual(['claude', 'codex']);
    expect(two.querySelector('.provider-icon')?.getAttribute('src')).toBe('/assets/agents/claude.png');
    expect(two.querySelector('[class*="page-dot"], [class*="pagination"], button')).toBeNull();
  });

  it('preserves an unknown window with a track but no fill', () => {
    const document = render(data([panel('codex', [{ name: '', windows: [window('unknown', 'Weekly', null, '7d')] }])]));
    expect(document.querySelectorAll('.ring-track')).toHaveLength(2);
    expect(document.querySelectorAll('.ring-fill')).toHaveLength(0);
    expect(document.querySelector('.meter-value strong')?.textContent).toBe('—');
  });

  it.each(['Waiting for Hub', 'Waiting for local usage'])('keeps %s within the compact viewport', (message) => {
    const document = render(data([], false, message));
    expect(document.querySelector('.waiting')?.textContent).toBe(message);
    expect(document.querySelector('.provider-card')).toBeNull();
    expect(document.querySelector('.token-count')).toBeNull();
    expect(document.querySelector('.waiting')?.closest('.compact-screen')).not.toBeNull();
  });

  it('handles empty limits and escapes source-supplied text', () => {
    expect(render(data([])).querySelector('.empty-limits')?.textContent).toBe('No usage limits');
    const document = render(data([panel('<untrusted>', [{ name: '', windows: [window('a', '<script>x</script>', 10)] }])]));
    expect(document.querySelector('.provider-name')?.textContent).toBe('<untrusted>');
    expect(document.querySelector('.original-label')?.textContent).toBe('<script>x</script>');
    expect(document.querySelector('script')).toBeNull();
  });

  it('selects ring radius by flattened DOM order, not nested @first', () => {
    expect(stylesheet).toContain('.window-ring:nth-of-type(2) .ring-inner');
    expect(stylesheet).toContain('.window-ring:nth-of-type(n+2) .ring-outer');
    expect(stylesheet).toContain('.window-ring:nth-of-type(n+3)');
    expect(readFileSync(join(themesRoot, 'compact/template.hbs'), 'utf8')).not.toContain('{{#if @first}}M');
  });
});
