export type LanguageId = "node" | "csharp" | "go" | "powershell" | "python";
export type StarterKind = "offline" | "liveValidate" | "liveDeploy";

export interface LanguageSample {
  id: LanguageId;
  label: string;
  highlightLanguage: string;
  packageManager: string;
  runtime: string;
  install: string;
  registry: string;
  packageUrl: string;
  guideUrl: string;
  testCommand: string;
  offlineStarter: string;
  liveValidateStarter: string;
  liveDeployStarter: string;
  offlineSampleUrl: string;
  liveSampleUrl: string;
}
