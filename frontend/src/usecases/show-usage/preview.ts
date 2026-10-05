export type ProfileSize = { id: string; width: number; height: number };

// A missing profile can only happen while settings are still loading; preserve the legacy ratio then.
export function previewAspectRatio(profiles: readonly ProfileSize[], profileID?: string) {
  const profile = profiles.find(candidate => candidate.id === profileID);
  return profile ? `${profile.width} / ${profile.height}` : '1920 / 462';
}
