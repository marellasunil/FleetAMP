package main

// themeCSS maps the existing control-plane components onto neutral dark and
// light palettes. Blue remains an action accent rather than the page surface.
const themeCSS = `
:root{
  color-scheme:dark;
  --theme-bg:#0c0c0e;--theme-bg-top:#17171a;--theme-sidebar:#101012;
  --theme-panel:#171719;--theme-panel-alt:#121214;--theme-panel-raised:#1d1d20;
  --theme-border:#303035;--theme-border-soft:#26262a;
  --theme-text:#f1f1f3;--theme-muted:#a0a0aa;--theme-subtle:#777783;
  --theme-input:#101012;--theme-code:#151518;--theme-nav:#202024;
  --theme-button:#242429;--theme-button-text:#f4f4f5;
  --theme-success-bg:#102d24;--theme-success-border:#276c57;--theme-success-text:#72dfb9;
  --theme-warning-bg:#332513;--theme-warning-border:#765426;--theme-warning-text:#f6bd68;
  --theme-off-bg:#202024;--theme-off-border:#48484f;--theme-off-text:#b8b8c0;
  --theme-error-bg:#351619;--theme-error-border:#963d48;--theme-error-text:#ff9ba8;
  --bg:var(--theme-bg);--panel:var(--theme-panel);--panel2:var(--theme-panel-alt);
  --line:var(--theme-border);--muted:var(--theme-muted);--text:var(--theme-text);
}
:root[data-theme="light"]{
  color-scheme:light;
  --theme-bg:#f5f5f6;--theme-bg-top:#ffffff;--theme-sidebar:#ffffff;
  --theme-panel:#ffffff;--theme-panel-alt:#fafafa;--theme-panel-raised:#f0f0f2;
  --theme-border:#d7d7dc;--theme-border-soft:#e6e6e9;
  --theme-text:#18181b;--theme-muted:#60606a;--theme-subtle:#797982;
  --theme-input:#ffffff;--theme-code:#f4f4f6;--theme-nav:#eeeeF1;
  --theme-button:#f1f1f3;--theme-button-text:#202024;
  --theme-success-bg:#e8f7f1;--theme-success-border:#8bc9b3;--theme-success-text:#176b50;
  --theme-warning-bg:#fff4df;--theme-warning-border:#dfb66e;--theme-warning-text:#825715;
  --theme-off-bg:#f0f0f2;--theme-off-border:#c4c4ca;--theme-off-text:#55555f;
  --theme-error-bg:#fff0f1;--theme-error-border:#d98a93;--theme-error-text:#942f3c;
}
body{background:radial-gradient(circle at 55% 0,var(--theme-bg-top) 0,var(--theme-bg) 46%);color:var(--theme-text)}
.side{background:var(--theme-sidebar);border-color:var(--theme-border)}
.navlabel,.crumb,th{color:var(--theme-subtle)}
.navitem{color:var(--theme-muted);background:transparent}
.navitem:hover,.navitem.active{color:var(--theme-text);background:var(--theme-nav);box-shadow:inset 3px 0 var(--blue)}
.navicon,.metricnote{color:var(--theme-subtle)}
.soon{color:var(--theme-muted);background:var(--theme-panel-raised)}
.mode,.connection{background:var(--theme-panel-alt);border-color:var(--theme-border);color:var(--theme-text)}
.top,.cardhead,.callout,th,td,.groupstats,.driftitem{border-color:var(--theme-border-soft)}
.metric,.card{background:var(--theme-panel);border-color:var(--theme-border)}
.upcomingitem,.groupselector,.pipeline-stage{background:var(--theme-panel-alt);border-color:var(--theme-border)}
.btn{background:var(--theme-button);border-color:var(--theme-border);color:var(--theme-button-text)}
.btn.primary{background:#4169d8;border-color:#3158c2;color:#fff}
.select,.input,.configeditor textarea{background:var(--theme-input);border-color:var(--theme-border);color:var(--theme-text)}
.healthbar{background:var(--theme-panel-raised)}
.badge.ok{color:var(--theme-success-text);border-color:var(--theme-success-border);background:var(--theme-success-bg)}
.badge.warn{color:var(--theme-warning-text);border-color:var(--theme-warning-border);background:var(--theme-warning-bg)}
.badge.off{color:var(--theme-off-text);border-color:var(--theme-off-border);background:var(--theme-off-bg)}
.chip{background:var(--theme-panel-raised);border-color:var(--theme-border)}
.configerror{background:var(--theme-error-bg);border-color:var(--theme-error-border);color:var(--theme-error-text)}
.autherror{background:var(--theme-error-bg);border-color:var(--theme-error-border);color:var(--theme-error-text)}
.notice{background:var(--theme-warning-bg);border-color:var(--theme-warning-border);color:var(--theme-warning-text)}
pre{background:var(--theme-code);border-color:var(--theme-border);color:var(--theme-text)}
.code{color:var(--blue)}
.theme-toggle{width:100%;border:0;font:inherit;text-align:left;cursor:pointer}
:root[data-theme="light"] .brandmark{box-shadow:0 6px 20px #536ed02e}
`

const themeJS = `(() => {
  const storageKey = "fleetamp-theme";
  const root = document.documentElement;
  const button = document.getElementById("theme-toggle");
  const saved = localStorage.getItem(storageKey);
  const initial = saved === "light" || saved === "dark" ? saved : "dark";

  const apply = (theme, persist) => {
    root.dataset.theme = theme;
    root.style.colorScheme = theme;
    if (persist) localStorage.setItem(storageKey, theme);
    if (button) {
      const next = theme === "dark" ? "light" : "dark";
      button.textContent = next === "light" ? "☀ Light theme" : "☾ Dark theme";
      button.setAttribute("aria-label", "Switch to " + next + " theme");
      button.setAttribute("aria-pressed", String(theme === "dark"));
    }
  };

  apply(initial, false);
  if (button) {
    button.addEventListener("click", () => {
      apply(root.dataset.theme === "dark" ? "light" : "dark", true);
    });
  }
})();`
