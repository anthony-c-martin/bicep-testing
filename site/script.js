const tabs = [...document.querySelectorAll('[role="tab"]')];
const panels = [...document.querySelectorAll('[role="tabpanel"]')];

function activateTab(tab) {
  const language = tab.dataset.language;

  for (const candidate of tabs) {
    const selected = candidate === tab;
    candidate.setAttribute('aria-selected', selected.toString());
    candidate.tabIndex = selected ? 0 : -1;
  }

  for (const panel of panels) {
    panel.hidden = panel.dataset.panel !== language;
  }
}

for (const [index, tab] of tabs.entries()) {
  tab.addEventListener('click', () => activateTab(tab));
  tab.addEventListener('keydown', event => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      return;
    }

    event.preventDefault();
    let nextIndex = index;
    if (event.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.length) % tabs.length;
    if (event.key === 'ArrowRight') nextIndex = (index + 1) % tabs.length;
    if (event.key === 'Home') nextIndex = 0;
    if (event.key === 'End') nextIndex = tabs.length - 1;
    activateTab(tabs[nextIndex]);
    tabs[nextIndex].focus();
  });
}

for (const button of document.querySelectorAll('.copy-button')) {
  button.addEventListener('click', async () => {
    await navigator.clipboard.writeText(button.dataset.copy);
    button.classList.add('copied');
    button.setAttribute('aria-label', 'Copied');
    button.innerHTML = '<i data-lucide="check" aria-hidden="true"></i>';
    window.lucide?.createIcons();

    window.setTimeout(() => {
      button.classList.remove('copied');
      button.setAttribute('aria-label', 'Copy install command');
      button.innerHTML = '<i data-lucide="copy" aria-hidden="true"></i>';
      window.lucide?.createIcons();
    }, 1800);
  });
}

const scene = document.querySelector('.hero-scene');
const hero = document.querySelector('.hero');
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

if (scene && hero && !reduceMotion && window.matchMedia('(pointer: fine)').matches) {
  hero.addEventListener('pointermove', event => {
    const bounds = hero.getBoundingClientRect();
    const horizontal = (event.clientX - bounds.left) / bounds.width - 0.5;
    const vertical = (event.clientY - bounds.top) / bounds.height - 0.5;
    scene.style.transform = `perspective(1100px) rotateY(${-4 + horizontal * 2}deg) rotateX(${1 - vertical * 2}deg)`;
  });

  hero.addEventListener('pointerleave', () => {
    scene.style.transform = '';
  });
}

window.addEventListener('DOMContentLoaded', () => window.lucide?.createIcons());