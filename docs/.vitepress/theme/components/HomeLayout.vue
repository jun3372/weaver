<script setup lang="ts">
import { useData, withBase } from 'vitepress'
import { computed } from 'vue'

type Link = { text: string; link: string; theme?: string; external?: boolean }

const { frontmatter, isDark } = useData()

const nav = computed<Link[]>(() => frontmatter.value.nav ?? [])
const hero = computed(() => frontmatter.value.hero ?? {})
const steps = computed<any[]>(() => frontmatter.value.steps ?? [])
const features = computed<any[]>(() => frontmatter.value.features ?? [])
const examples = computed(() => frontmatter.value.examples ?? {})
const footer = computed(() => frontmatter.value.footer ?? {})
const altLang = computed(() => frontmatter.value.altLang ?? null)

function toggleDark() {
  isDark.value = !isDark.value
}

const href = (link: string) => (link.startsWith('/') ? withBase(link) : link)

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

function highlight(code: string, lang: string): string {
  let html = esc(code)
  if (lang === 'go') {
    html = html
      .replace(/(\/\/[^\n]*)/g, '<span class="tok-com">$1</span>')
      .replace(/("[^"\n]*")/g, '<span class="tok-str">$1</span>')
      .replace(/\b(type|interface|struct|func|return|package|import)\b/g, '<span class="tok-kw">$1</span>')
      .replace(/\b(string|error|int|bool|context)\b/g, '<span class="tok-ty">$1</span>')
  } else {
    html = html
      .replace(/^(\$)/gm, '<span class="tok-prompt">$1</span>')
      .replace(/(#[^\n]*)/g, '<span class="tok-com">$1</span>')
  }
  return html
}
</script>

<template>
  <div class="home">
    <header class="home-nav">
      <div class="home-container nav-inner">
        <a class="nav-brand" :href="href('/')">
          <img :src="withBase('/logo.svg')" alt="" class="nav-logo" />
          <span>Weaver</span>
        </a>
        <nav class="nav-links" aria-label="主导航">
          <a v-for="item in nav" :key="item.link" :href="href(item.link)">{{ item.text }}</a>
        </nav>
        <div class="nav-actions">
          <a v-if="altLang" class="nav-lang" :href="href(altLang.link)">{{ altLang.text }}</a>
          <a
            class="nav-github"
            href="https://github.com/jun3372/weaver"
            aria-label="GitHub"
            target="_blank"
            rel="noopener noreferrer"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path
                d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.55v-2.17c-3.2.7-3.87-1.36-3.87-1.36-.53-1.33-1.28-1.69-1.28-1.69-1.05-.72.08-.7.08-.7 1.16.08 1.77 1.19 1.77 1.19 1.03 1.77 2.7 1.26 3.36.96.1-.75.4-1.26.73-1.55-2.55-.29-5.24-1.28-5.24-5.69 0-1.26.45-2.29 1.19-3.09-.12-.29-.52-1.46.11-3.05 0 0 .97-.31 3.18 1.18a11.1 11.1 0 0 1 5.8 0c2.2-1.49 3.17-1.18 3.17-1.18.63 1.59.23 2.76.11 3.05.74.8 1.19 1.83 1.19 3.09 0 4.42-2.7 5.39-5.26 5.68.41.36.78 1.06.78 2.14v3.17c0 .3.21.67.8.55A10.52 10.52 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5Z"
              />
            </svg>
          </a>
          <button class="nav-toggle" type="button" :aria-label="isDark ? '切换到浅色模式' : '切换到深色模式'" @click="toggleDark">
            <svg v-if="isDark" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
              <circle cx="12" cy="12" r="4" />
              <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
            </svg>
            <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
            </svg>
          </button>
        </div>
      </div>
    </header>

    <main>
      <section class="hero">
        <div class="home-container hero-grid">
          <div class="hero-copy">
            <p v-if="hero.badge" class="hero-badge">{{ hero.badge }}</p>
            <h1 class="hero-title">{{ hero.title }}</h1>
            <p class="hero-tagline">{{ hero.tagline }}</p>
            <div class="hero-actions">
              <a
                v-for="action in hero.actions"
                :key="action.link"
                class="hero-btn"
                :class="action.theme === 'brand' ? 'is-brand' : 'is-alt'"
                :href="href(action.link)"
                :target="action.external ? '_blank' : undefined"
                :rel="action.external ? 'noopener noreferrer' : undefined"
              >
                {{ action.text }}
              </a>
            </div>
          </div>
          <div v-if="hero.terminal" class="hero-terminal" aria-hidden="true">
            <div class="terminal-bar">
              <span class="dot dot-r"></span>
              <span class="dot dot-y"></span>
              <span class="dot dot-g"></span>
              <span class="terminal-title">{{ hero.terminal.title }}</span>
            </div>
            <pre class="terminal-body"><code><span
              v-for="(line, i) in hero.terminal.lines"
              :key="i"
              class="terminal-line"
              :class="line.t"
              :style="{ animationDelay: 0.15 + i * 0.08 + 's' }"
            >{{ line.s }}</span></code></pre>
          </div>
        </div>
      </section>

      <section v-if="steps.length" class="steps">
        <div class="home-container">
          <h2 class="section-title">{{ frontmatter.stepsTitle }}</h2>
          <ol class="steps-grid">
            <li v-for="(step, i) in steps" :key="i" class="step-card">
              <span class="step-num">{{ i + 1 }}</span>
              <h3>{{ step.title }}</h3>
              <p>{{ step.details }}</p>
              <pre v-if="step.code" class="step-code"><code v-html="highlight(step.code, step.lang)"></code></pre>
            </li>
          </ol>
        </div>
      </section>

      <section v-if="features.length" class="features">
        <div class="home-container">
          <h2 class="section-title">{{ frontmatter.featuresTitle }}</h2>
          <div class="features-grid">
            <article v-for="f in features" :key="f.title" class="feature-card">
              <div class="feature-icon" v-html="f.icon"></div>
              <h3>{{ f.title }}</h3>
              <p>{{ f.details }}</p>
            </article>
          </div>
        </div>
      </section>

      <section v-if="examples.items?.length" class="examples">
        <div class="home-container">
          <h2 class="section-title">{{ examples.title }}</h2>
          <p v-if="examples.details" class="section-desc">{{ examples.details }}</p>
          <div class="examples-grid">
            <a v-for="ex in examples.items" :key="ex.name" class="example-card" :href="href(ex.link)">
              <span class="example-tag">{{ ex.tag }}</span>
              <h3><code>{{ ex.name }}</code> · {{ ex.title }}</h3>
              <p>{{ ex.details }}</p>
              <span class="example-arrow" aria-hidden="true">→</span>
            </a>
          </div>
        </div>
      </section>
    </main>

    <footer class="home-footer">
      <div class="home-container">
        <p v-if="footer.message" class="footer-message">{{ footer.message }}</p>
        <p v-if="footer.copyright" class="footer-copyright">{{ footer.copyright }}</p>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.home {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
  font-family: var(--vp-font-family-base);
}

.home-container {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

a {
  text-decoration: none;
}

a:focus-visible,
button:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 3px;
  border-radius: 6px;
}

/* ---------- Nav ---------- */

.home-nav {
  position: sticky;
  top: 0;
  z-index: 100;
  background: color-mix(in srgb, var(--vp-c-bg) 82%, transparent);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--vp-c-divider);
}

.nav-inner {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 28px;
}

.nav-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
}

.nav-logo {
  width: 26px;
  height: 26px;
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
}

.nav-links::-webkit-scrollbar {
  display: none;
}

.nav-links a {
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
  white-space: nowrap;
  transition: color 0.2s ease, background-color 0.2s ease;
}

.nav-links a:hover {
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.nav-lang {
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
  padding: 6px 10px;
  border-radius: 8px;
  transition: color 0.2s ease;
}

.nav-lang:hover {
  color: var(--vp-c-brand-1);
}

.nav-github,
.nav-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--vp-c-text-2);
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease;
}

.nav-github:hover,
.nav-toggle:hover {
  color: var(--vp-c-text-1);
  background: var(--vp-c-bg-soft);
}

/* ---------- Hero ---------- */

.hero {
  padding: 88px 0 72px;
  overflow: hidden;
  background:
    radial-gradient(ellipse 60% 50% at 70% 0%, var(--vp-c-brand-soft), transparent 70%);
}

.hero-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 48px;
  align-items: center;
}

@media (min-width: 1024px) {
  .hero-grid {
    grid-template-columns: minmax(0, 1fr) minmax(0, 480px);
  }
}

.hero-badge {
  display: inline-block;
  padding: 5px 14px;
  border: 1px solid var(--vp-c-brand-1);
  border-radius: 999px;
  background: var(--vp-c-brand-soft);
  color: var(--vp-c-brand-1);
  font-size: 0.8rem;
  font-weight: 600;
  font-family: var(--vp-font-family-mono);
}

.hero-title {
  margin: 20px 0 16px;
  font-size: clamp(2rem, 4.6vw, 2.9rem);
  line-height: 1.15;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.hero-tagline {
  margin: 0 0 32px;
  font-size: 1.05rem;
  line-height: 1.7;
  color: var(--vp-c-text-2);
  max-width: 34em;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.hero-btn {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 0 24px;
  border-radius: 10px;
  font-size: 0.95rem;
  font-weight: 600;
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease, transform 0.15s ease;
}

.hero-btn:active {
  transform: scale(0.97);
}

.hero-btn.is-brand {
  background: var(--vp-button-brand-bg);
  color: var(--vp-button-brand-text);
  border: 1px solid var(--vp-button-brand-border);
}

.hero-btn.is-brand:hover {
  background: var(--vp-button-brand-hover-bg);
  border-color: var(--vp-button-brand-hover-border);
}

.hero-btn.is-alt {
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
  border: 1px solid var(--vp-c-divider);
}

.hero-btn.is-alt:hover {
  color: var(--vp-c-brand-1);
  border-color: var(--vp-c-brand-1);
}

/* ---------- Terminal ---------- */

.hero-terminal {
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: #0f172a;
  box-shadow: 0 16px 40px -16px rgba(2, 6, 23, 0.4);
  overflow: hidden;
}

.terminal-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid rgba(148, 163, 184, 0.15);
}

.dot {
  width: 11px;
  height: 11px;
  border-radius: 50%;
}

.dot-r { background: #f87171; }
.dot-y { background: #fbbf24; }
.dot-g { background: #34d399; }

.terminal-title {
  margin-left: 8px;
  font-size: 0.78rem;
  color: #8b949e;
  font-family: var(--vp-font-family-mono);
}

.terminal-body {
  margin: 0;
  padding: 20px 20px 24px;
  font-family: var(--vp-font-family-mono);
  font-size: 0.85rem;
  line-height: 1.9;
  color: #e1e4e8;
  overflow-x: auto;
}

.terminal-line {
  display: block;
  white-space: pre;
  opacity: 0;
  animation: terminal-in 0.35s ease-out forwards;
}

.terminal-line.cmd::before {
  content: '$ ';
  color: #4ade80;
}

.terminal-line.cmd {
  color: #e1e4e8;
}

.terminal-line.out {
  color: #8b949e;
}

.terminal-line.ok {
  color: #4ade80;
}

.terminal-line.info {
  color: #a5d6ff;
}

@keyframes terminal-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ---------- Sections ---------- */

.section-title {
  margin: 0 0 12px;
  font-size: 1.6rem;
  font-weight: 700;
  text-align: center;
}

.section-desc {
  margin: 0 auto 40px;
  max-width: 36em;
  text-align: center;
  color: var(--vp-c-text-2);
  line-height: 1.7;
}

/* ---------- Steps ---------- */

.steps {
  padding: 56px 0;
}

.steps-grid {
  list-style: none;
  margin: 40px 0 0;
  padding: 0;
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
  counter-reset: step;
}

@media (min-width: 768px) {
  .steps-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

.step-card {
  position: relative;
  padding: 24px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  background: var(--vp-c-bg);
  transition: border-color 0.2s ease;
}

.step-card:hover {
  border-color: var(--vp-c-brand-1);
}

.step-num {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--vp-c-brand-soft);
  color: var(--vp-c-brand-1);
  font-family: var(--vp-font-family-mono);
  font-weight: 700;
  font-size: 0.9rem;
}

.step-card h3 {
  margin: 14px 0 8px;
  font-size: 1.05rem;
}

.step-card p {
  margin: 0 0 16px;
  font-size: 0.9rem;
  line-height: 1.65;
  color: var(--vp-c-text-2);
}

.step-code {
  margin: 0;
  padding: 14px 16px;
  border-radius: 8px;
  background: #0f172a;
  color: #e1e4e8;
  font-family: var(--vp-font-family-mono);
  font-size: 0.78rem;
  line-height: 1.7;
  overflow-x: auto;
}

/* ---------- Features ---------- */

.features {
  padding: 56px 0;
  background: var(--vp-c-bg-soft);
}

.features-grid {
  margin-top: 40px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

@media (min-width: 640px) {
  .features-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (min-width: 1024px) {
  .features-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

.feature-card {
  padding: 24px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  background: var(--vp-c-bg);
  transition: border-color 0.2s ease;
}

.feature-card:hover {
  border-color: var(--vp-c-brand-1);
}

.feature-icon {
  display: flex;
  align-items: center;
  color: var(--vp-c-brand-1);
  width: 52px;
  height: 52px;
}

.feature-icon :deep(svg) {
  width: 30px;
  height: 30px;
}

.feature-card h3 {
  margin: 14px 0 8px;
  font-size: 1.05rem;
}

.feature-card p {
  margin: 0;
  font-size: 0.9rem;
  line-height: 1.65;
  color: var(--vp-c-text-2);
}

/* ---------- Examples ---------- */

.examples {
  padding: 56px 0 72px;
}

.examples-grid {
  margin-top: 36px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

@media (min-width: 768px) {
  .examples-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

.example-card {
  position: relative;
  display: block;
  padding: 24px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  background: var(--vp-c-bg);
  transition: border-color 0.2s ease;
}

.example-card:hover {
  border-color: var(--vp-c-brand-1);
}

.example-tag {
  display: inline-block;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--vp-c-brand-soft);
  color: var(--vp-c-brand-1);
  font-size: 0.75rem;
  font-weight: 600;
}

.example-card h3 {
  margin: 14px 0 8px;
  font-size: 1.02rem;
}

.example-card h3 code {
  font-family: var(--vp-font-family-mono);
  color: var(--vp-c-brand-1);
  font-size: 0.95em;
}

.example-card p {
  margin: 0 0 16px;
  font-size: 0.9rem;
  line-height: 1.65;
  color: var(--vp-c-text-2);
}

.example-arrow {
  position: absolute;
  right: 20px;
  bottom: 20px;
  color: var(--vp-c-brand-1);
  font-weight: 600;
  transition: transform 0.2s ease;
}

.example-card:hover .example-arrow {
  transform: translateX(4px);
}

/* ---------- Footer ---------- */

.home-footer {
  margin-top: auto;
  padding: 28px 0;
  border-top: 1px solid var(--vp-c-divider);
  text-align: center;
}

.footer-message {
  margin: 0;
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
}

.footer-copyright {
  margin: 6px 0 0;
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
}

/* ---------- Code tokens (v-html 内容无 data-v 属性，需 :deep) ---------- */

.step-code :deep(.tok-com) { color: #8b949e; }
.step-code :deep(.tok-str) { color: #a5d6ff; }
.step-code :deep(.tok-kw) { color: #ff7b72; }
.step-code :deep(.tok-ty) { color: #79c0ff; }
.step-code :deep(.tok-prompt) { color: #4ade80; user-select: none; }

/* ---------- Reduced motion ---------- */

@media (prefers-reduced-motion: reduce) {
  .terminal-line {
    animation: none;
    opacity: 1;
  }

  .hero-btn,
  .step-card,
  .feature-card,
  .example-card,
  .example-arrow {
    transition: none;
  }
}
</style>
