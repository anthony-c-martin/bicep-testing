import { useState, type KeyboardEvent } from "react";
import { ArrowDown, ArrowUpRight, Check, Copy, GitFork } from "lucide-react";
import hljs from "highlight.js/lib/core";
import csharp from "highlight.js/lib/languages/csharp";
import go from "highlight.js/lib/languages/go";
import javascript from "highlight.js/lib/languages/javascript";
import powershell from "highlight.js/lib/languages/powershell";
import python from "highlight.js/lib/languages/python";
import nodeOffline from "../../samples/node/snapshot.test.js?raw";
import nodeLive from "../../samples/node/deployment.test.js?raw";
import csharpOffline from "../../samples/dotnet/OfflineTests.cs?raw";
import csharpLive from "../../samples/dotnet/LiveTests.cs?raw";
import goOffline from "../../samples/go/snapshot_test.go?raw";
import goLive from "../../samples/go/deployment_test.go?raw";
import powershellOffline from "../../samples/powershell/BicepTest.Sample.Tests.ps1?raw";
import powershellLive from "../../samples/powershell/BicepTest.Deployment.Sample.Tests.ps1?raw";
import pythonOffline from "../../samples/python/test_snapshot.py?raw";
import pythonLive from "../../samples/python/test_deployment.py?raw";

const repositoryUrl = "https://github.com/anthony-c-martin/bicep-testing";

type LanguageId = "node" | "csharp" | "go" | "powershell" | "python";
type SampleKind = "offline" | "live";

interface LanguageSample {
  id: LanguageId;
  label: string;
  highlightLanguage: string;
  packageManager: string;
  runtime: string;
  install: string;
  registry: string;
  packageUrl: string;
  offlineName: string;
  liveName: string;
  offline: string;
  live: string;
}

interface LanguageTabsProps {
  selected: LanguageId;
  onSelect: (language: LanguageId) => void;
  label: string;
}

interface CopyButtonProps {
  value: string;
  label: string;
}

hljs.registerLanguage("csharp", csharp);
hljs.registerLanguage("go", go);
hljs.registerLanguage("javascript", javascript);
hljs.registerLanguage("powershell", powershell);
hljs.registerLanguage("python", python);

const languages: readonly LanguageSample[] = [
  {
    id: "node",
    label: "Node",
    highlightLanguage: "javascript",
    packageManager: "npm",
    runtime: "Node.js 22+",
    install: "npm install --save-dev @anthony-c-martin/bicep-testing",
    registry: "npm",
    packageUrl: "https://www.npmjs.com/package/@anthony-c-martin/bicep-testing",
    offlineName: "snapshot.test.js",
    liveName: "deployment.test.js",
    offline: nodeOffline,
    live: nodeLive,
  },
  {
    id: "csharp",
    label: "C#",
    highlightLanguage: "csharp",
    packageManager: "NuGet",
    runtime: ".NET 10+",
    install: "dotnet add package AnthonyCMartin.BicepTesting",
    registry: "NuGet",
    packageUrl: "https://www.nuget.org/packages/AnthonyCMartin.BicepTesting",
    offlineName: "OfflineTests.cs",
    liveName: "LiveTests.cs",
    offline: csharpOffline,
    live: csharpLive,
  },
  {
    id: "go",
    label: "Go",
    highlightLanguage: "go",
    packageManager: "Go modules",
    runtime: "Go 1.25+",
    install:
      "go get github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing",
    registry: "pkg.go.dev",
    packageUrl:
      "https://pkg.go.dev/github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing",
    offlineName: "snapshot_test.go",
    liveName: "deployment_test.go",
    offline: goOffline,
    live: goLive,
  },
  {
    id: "powershell",
    label: "PowerShell",
    highlightLanguage: "powershell",
    packageManager: "PSGallery",
    runtime: "PowerShell 7.6+",
    install: "Install-PSResource AnthonyCMartin.BicepTesting",
    registry: "PowerShell Gallery",
    packageUrl:
      "https://www.powershellgallery.com/packages/AnthonyCMartin.BicepTesting",
    offlineName: "BicepTest.Sample.Tests.ps1",
    liveName: "BicepTest.Deployment.Sample.Tests.ps1",
    offline: powershellOffline,
    live: powershellLive,
  },
  {
    id: "python",
    label: "Python",
    highlightLanguage: "python",
    packageManager: "PyPI",
    runtime: "Python 3.11+",
    install: "python -m pip install anthonycmartin-bicep-testing",
    registry: "PyPI",
    packageUrl: "https://pypi.org/project/anthonycmartin-bicep-testing/",
    offlineName: "test_snapshot.py",
    liveName: "test_deployment.py",
    offline: pythonOffline,
    live: pythonLive,
  },
];

function getLanguage(id: LanguageId): LanguageSample {
  return languages.find((language) => language.id === id)!;
}

function highlight(code: string, language: string): string {
  return hljs.highlight(code, { language }).value;
}

function LanguageTabs({ selected, onSelect, label }: LanguageTabsProps) {
  function handleKeyDown(
    event: KeyboardEvent<HTMLButtonElement>,
    index: number,
  ) {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const nextIndex =
      event.key === "Home"
        ? 0
        : event.key === "End"
          ? languages.length - 1
          : (index + (event.key === "ArrowRight" ? 1 : -1) + languages.length) %
            languages.length;
    onSelect(languages[nextIndex].id);
    event.currentTarget.parentElement
      ?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
      [nextIndex]?.focus();
  }

  return (
    <div className="language-tabs" role="tablist" aria-label={label}>
      {languages.map((language, index) => (
        <button
          key={language.id}
          type="button"
          role="tab"
          aria-selected={selected === language.id}
          tabIndex={selected === language.id ? 0 : -1}
          onClick={() => onSelect(language.id)}
          onKeyDown={(event) => handleKeyDown(event, index)}
        >
          {language.label}
        </button>
      ))}
    </div>
  );
}

function CopyButton({ value, label }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  }
  return (
    <button
      className="icon-button"
      type="button"
      onClick={copy}
      aria-label={copied ? "Copied" : label}
      title={copied ? "Copied" : label}
    >
      {copied ? <Check /> : <Copy />}
    </button>
  );
}

export default function App() {
  const [selectedLanguage, setSelectedLanguage] = useState<LanguageId>("node");
  const [sampleKind, setSampleKind] = useState<SampleKind>("offline");
  const language = getLanguage(selectedLanguage);
  const sampleCode = language[sampleKind];
  const sampleName =
    sampleKind === "offline" ? language.offlineName : language.liveName;
  const highlightedSample = highlight(sampleCode, language.highlightLanguage);

  return (
    <>
      <header className="site-header">
        <a className="brand" href="#top">
          <img
            src="/bicep-testing/bicep-logo.svg"
            alt=""
            width="34"
            height="34"
          />
          <span>Bicep Testing Framework</span>
        </a>
        <nav aria-label="Primary navigation">
          <a href="#install">Install</a>
          <a href="#samples">Samples</a>
          <a className="github-link" href={repositoryUrl}>
            <GitFork />
            <span>GitHub</span>
          </a>
        </nav>
      </header>
      <main id="top">
        <section className="hero">
          <div className="hero-content">
            <div className="hero-intro">
              <p className="eyebrow">
                <span /> Language-agnostic Bicep Testing
              </p>
              <h1>Bicep Testing Framework</h1>
              <p className="hero-lede">
                Language-native libraries for testing Bicep files from Jest,
                MSTest, Go testing, Pester, or pytest. Tests can inspect planned
                infrastructure locally or validate and deploy resources in Azure.
              </p>
              <div className="hero-actions">
                <a className="button button-primary" href="#install">
                  Install the library <ArrowDown />
                </a>
                <a className="button button-secondary" href="#samples">
                  View test samples <ArrowDown />
                </a>
              </div>
            </div>
            <div className="hero-details">
              <div className="hero-use-cases">
                <article>
                  <span>01 / Offline tests</span>
                  <h2>Check planned infrastructure locally</h2>
                  <p>
                    Compile a <code>.bicepparam</code> file and assert on
                    predicted resources, outputs, and diagnostics without Azure
                    credentials or a deployment.
                  </p>
                </article>
                <article>
                  <span>02 / Live tests</span>
                  <h2>Verify real Azure behavior</h2>
                  <p>
                    Validate templates against Azure or deploy with an Azure
                    Deployment Stack, inspect service responses, and clean up
                    managed test resources.
                  </p>
                </article>
              </div>
              <div className="hero-browse">
                <strong>Viewing examples</strong>
                <p>
                  Choose a language in the selector below. The install command,
                  guide link, and inline source all update together. Open the
                  Samples section and switch between Offline and Live to
                  compare both test styles.
                </p>
              </div>
            </div>
          </div>
        </section>
        <div className="global-language-switcher">
          <div className="global-language-switcher-inner">
            <span>Language</span>
            <LanguageTabs
              selected={selectedLanguage}
              onSelect={setSelectedLanguage}
              label="Site language"
            />
          </div>
        </div>
        <section className="install-section" id="install">
          <div className="section-heading">
            <p className="section-kicker">Install</p>
            <h2>Use your native test stack</h2>
            <p>
              Equivalent behavior, expressed through each ecosystem's
              conventions and lifecycle patterns.
            </p>
          </div>
          <div className="install-tool">
            <div className="install-panel" role="tabpanel">
              <div className="install-meta">
                <span>{language.packageManager}</span>
                <span>{language.runtime}</span>
              </div>
              <div className="command-row">
                <code>{language.install}</code>
                <CopyButton
                  value={language.install}
                  label={`Copy ${language.label} install command`}
                />
              </div>
              <div className="install-links">
                <a href={language.packageUrl}>
                  View on {language.registry} <ArrowUpRight />
                </a>
              </div>
            </div>
          </div>
        </section>
        <section className="samples-section" id="samples">
          <div className="section-heading">
            <p className="section-kicker">Runnable samples</p>
            <h2>The tests, in full</h2>
            <p>These are real, working test samples you can run yourself.</p>
          </div>
          <div className="sample-workbench">
            <div className="sample-toolbar">
              <div className="kind-switch" aria-label="Sample type">
                <button
                  type="button"
                  className={sampleKind === "offline" ? "active" : ""}
                  onClick={() => setSampleKind("offline")}
                >
                  Offline
                </button>
                <button
                  type="button"
                  className={sampleKind === "live" ? "active" : ""}
                  onClick={() => setSampleKind("live")}
                >
                  Live
                </button>
              </div>
              <span className="file-name">{sampleName}</span>
              <CopyButton value={sampleCode} label={`Copy ${sampleName}`} />
            </div>
            <pre className="sample-code">
              <code dangerouslySetInnerHTML={{ __html: highlightedSample }} />
            </pre>
          </div>
        </section>
      </main>
      <footer>
        <a className="brand" href="#top">
          <img
            src="/bicep-testing/bicep-logo.svg"
            alt=""
            width="28"
            height="28"
          />
          <span>Bicep Testing Framework</span>
        </a>
        <div>
          <a href={`${repositoryUrl}/blob/main/LICENSE`}>MIT License</a>
          <a href={`${repositoryUrl}/issues`}>Issues</a>
        </div>
      </footer>
    </>
  );
}
