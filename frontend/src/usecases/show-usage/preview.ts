export type ProfileSize = { id: string; width: number; height: number };

// Without profile metadata, let the image keep its intrinsic aspect ratio.
export function previewAspectRatio(profiles: readonly ProfileSize[], profileID?: string) {
  const profile = profiles.find(candidate => candidate.id === profileID);
  return profile && profile.width > 0 && profile.height > 0 ? `${profile.width} / ${profile.height}` : 'auto';
}

// Keep the image height equal to the full-width 1920x462 preview.
export function previewWidth(profiles: readonly ProfileSize[], profileID?: string) {
  const profile = profiles.find(candidate => candidate.id === profileID);
  return profile && profile.width > 0 && profile.height > 0
    ? `${(profile.width / profile.height) / (1920 / 462) * 100}%`
    : undefined;
}
