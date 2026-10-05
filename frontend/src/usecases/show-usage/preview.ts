export type ProfileSize = { id: string; width: number; height: number };

// Without profile metadata, let the image keep its intrinsic aspect ratio.
export function previewAspectRatio(profiles: readonly ProfileSize[], profileID?: string) {
  const profile = profiles.find(candidate => candidate.id === profileID);
  return profile && profile.width > 0 && profile.height > 0 ? `${profile.width} / ${profile.height}` : 'auto';
}
