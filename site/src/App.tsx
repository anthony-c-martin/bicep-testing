import { useState, type KeyboardEvent } from "react";
import {
  ArrowDown,
  ArrowUpRight,
  Check,
  Cloud,
  Copy,
  FileCode2,
  GitFork,
  Terminal,
} from "lucide-react";
import hljs from "highlight.js/lib/core";
import csharp from "highlight.js/lib/languages/csharp";
import go from "highlight.js/lib/languages/go";
import javascript from "highlight.js/lib/languages/javascript";
import powershell from "highlight.js/lib/languages/powershell";
import python from "highlight.js/lib/languages/python";
import {
  getLanguage,
  languages,
  repositoryUrl,
  type LanguageId,
  type StarterKind,
} from "./languages";

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
  const [starterKind, setStarterKind] = useState<StarterKind>("offline");
  const language = getLanguage(selectedLanguage);
  const starterCode = {
    offline: language.offlineStarter,
    liveValidate: language.liveValidateStarter,
    liveDeploy: language.liveDeployStarter,
  }[starterKind];
  const highlightedStarter = highlight(starterCode, language.highlightLanguage);

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
          <a href="#getting-started">Getting started</a>
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
                infrastructure locally or validate and deploy resources in
                Azure.
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
                  quickstarts, guide link, and sample links all update together.
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
        <section className="getting-started-section" id="getting-started">
          <div className="section-heading">
            <p className="section-kicker">Getting started</p>
            <h2>From package to first assertion</h2>
            <p>
              Start offline with no Azure credentials, then switch to a live
              validation when the template needs an Azure preflight check.
            </p>
          </div>
          <div className="getting-started-layout">
            <ol className="getting-started-steps">
              <li>
                <Terminal aria-hidden="true" />
                <div>
                  <span>01</span>
                  <strong>Install for {language.label}</strong>
                  <code>{language.install}</code>
                </div>
              </li>
              <li>
                <FileCode2 aria-hidden="true" />
                <div>
                  <span>02</span>
                  <strong>Create a test</strong>
                  <p>
                    Point the session at a checked-in <code>.bicepparam</code>{" "}
                    file.
                  </p>
                </div>
              </li>
              <li>
                <Cloud aria-hidden="true" />
                <div>
                  <span>03</span>
                  <strong>Run it natively</strong>
                  <code>{language.testCommand}</code>
                </div>
              </li>
            </ol>
            <div className="starter-workbench">
              <div className="starter-toolbar">
                <div
                  className="kind-switch"
                  aria-label="Getting started test type"
                >
                  <button
                    type="button"
                    className={starterKind === "offline" ? "active" : ""}
                    onClick={() => setStarterKind("offline")}
                  >
                    Offline
                  </button>
                  <button
                    type="button"
                    className={starterKind === "liveValidate" ? "active" : ""}
                    onClick={() => setStarterKind("liveValidate")}
                  >
                    Live (Validate)
                  </button>
                  <button
                    type="button"
                    className={starterKind === "liveDeploy" ? "active" : ""}
                    onClick={() => setStarterKind("liveDeploy")}
                  >
                    Live (Deploy)
                  </button>
                </div>
                <span>{language.label} quickstart</span>
                <CopyButton
                  value={starterCode}
                  label={`Copy ${language.label} ${starterKind} quickstart`}
                />
              </div>
              <pre className="starter-code">
                <code
                  dangerouslySetInnerHTML={{ __html: highlightedStarter }}
                />
              </pre>
              <a className="starter-guide-link" href={language.guideUrl}>
                Read the complete {language.label} guide <ArrowUpRight />
              </a>
            </div>
          </div>
        </section>
        <section className="samples-section" id="samples">
          <div className="section-heading">
            <p className="section-kicker">Runnable samples</p>
            <h2>Explore the complete tests</h2>
            <p>
              Open the real {language.label} test files in GitHub to see the
              complete setup, assertions, and cleanup flow.
            </p>
          </div>
          <div className="sample-links">
            <a href={language.offlineSampleUrl}>
              <span>01 / Offline</span>
              <strong>Local snapshot tests</strong>
              <p>
                Inspect predicted resources, outputs, and diagnostics without
                Azure credentials.
              </p>
              <span className="sample-link-action">
                View on GitHub <ArrowUpRight />
              </span>
            </a>
            <a href={language.liveSampleUrl}>
              <span>02 / Live</span>
              <strong>Azure validation and deployment tests</strong>
              <p>
                Validate against Azure, deploy real resources, assert behavior,
                and tear everything down.
              </p>
              <span className="sample-link-action">
                View on GitHub <ArrowUpRight />
              </span>
            </a>
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
