import { describe, expect, it } from 'vitest';
import { previewAspectRatio } from '../../src/usecases/show-usage/preview';

const profiles = [
  { id: 'ultra-wide-1920x462', width: 1920, height: 462 },
  { id: 'compact-gauge-480x320', width: 480, height: 320 },
];

describe('previewAspectRatio', () => {
  it('uses the selected profile dimensions', () => {
    expect(previewAspectRatio(profiles, 'ultra-wide-1920x462')).toBe('1920 / 462');
    expect(previewAspectRatio(profiles, 'compact-gauge-480x320')).toBe('480 / 320');
  });

  it('preserves the legacy ratio while profile settings are unavailable', () => {
    expect(previewAspectRatio([], undefined)).toBe('1920 / 462');
  });
});
