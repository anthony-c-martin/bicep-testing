import { csharp } from "./csharp";
import { go } from "./go";
import { node } from "./node";
import { powershell } from "./powershell";
import { python } from "./python";
import type { LanguageId, LanguageSample } from "./types";

export { repositoryUrl } from "./constants";
export type { LanguageId, LanguageSample, StarterKind } from "./types";

export const languages: readonly LanguageSample[] = [
  node,
  csharp,
  go,
  powershell,
  python,
];

export function getLanguage(id: LanguageId): LanguageSample {
  return languages.find((language) => language.id === id)!;
}
