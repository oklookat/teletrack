package html

import "fmt"

// indexHTML returns the public status page. apiPrefix is the path under which
// the embedded Teletrack API is mounted (e.g. "/api/v1/teletrack").
func indexHTML(apiPrefix string) string {
	if apiPrefix == "" {
		apiPrefix = "/api/v1/teletrack"
	}
	return fmt.Sprintf(indexHTMLTemplate, apiPrefix, apiPrefix)
}

// indexHTMLTemplate uses %s placeholders for the API path prefix
// (playing endpoint, then events endpoint).
const indexHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"/>
<meta name="robots" content="noindex"/>
<meta name="color-scheme" content="dark light"/>
<title>teletrack</title>
<style>
  :root {
    color-scheme: dark light;
    --bg: #0b0d10;
    --bg-accent: #12151a;
    --card: #161a21;
    --card-border: rgba(255,255,255,.06);
    --text: #eef0f3;
    --muted: #9aa3af;
    --accent: #7aa2f7;
    --accent-soft: rgba(122,162,247,.15);
    --bar: #2a303a;
    --bar-fill: #7aa2f7;
    --radius: 18px;
    --shadow: 0 12px 40px rgba(0,0,0,.35);
    --safe-bottom: env(safe-area-inset-bottom, 0px);
  }
  @media (prefers-color-scheme: light) {
    :root {
      --bg: #f0f2f5;
      --bg-accent: #e6e9ee;
      --card: #ffffff;
      --card-border: rgba(0,0,0,.06);
      --text: #1a1d23;
      --muted: #5c6570;
      --accent: #2563eb;
      --accent-soft: rgba(37,99,235,.12);
      --bar: #e5e8ed;
      --bar-fill: #2563eb;
      --shadow: 0 12px 32px rgba(15,23,42,.08);
    }
  }
  * { box-sizing: border-box; }
  html, body { height: 100%%; }
  body {
    margin: 0;
    min-height: 100%%;
    min-height: 100dvh;
    font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    background:
      radial-gradient(1200px 600px at 10%% -10%%, var(--accent-soft), transparent 55%%),
      radial-gradient(900px 500px at 100%% 0%%, var(--accent-soft), transparent 50%%),
      var(--bg);
    color: var(--text);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: max(1rem, env(safe-area-inset-top))
             max(1rem, env(safe-area-inset-right))
             max(1rem, var(--safe-bottom))
             max(1rem, env(safe-area-inset-left));
    -webkit-font-smoothing: antialiased;
  }
  .card {
    width: min(440px, 100%%);
    background: var(--card);
    border: 1px solid var(--card-border);
    border-radius: var(--radius);
    padding: 1.35rem 1.35rem 1.1rem;
    box-shadow: var(--shadow);
  }
  @media (min-width: 720px) {
    .card {
      width: min(520px, 100%%);
      padding: 1.5rem 1.6rem 1.2rem;
    }
  }
  @media (min-width: 960px) {
    .layout-wide .card {
      width: min(720px, 100%%);
    }
    .layout-wide .media {
      display: grid;
      grid-template-columns: 200px 1fr;
      gap: 1.25rem;
      align-items: start;
    }
    .layout-wide .cover-wrap {
      margin-bottom: 0;
    }
  }
  .clock {
    font-size: .85rem;
    color: var(--muted);
    margin-bottom: 1rem;
    font-variant-numeric: tabular-nums;
    letter-spacing: .02em;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: .35rem;
    font-size: .8rem;
    font-weight: 600;
    color: var(--accent);
    background: var(--accent-soft);
    padding: .28rem .6rem;
    border-radius: 999px;
    margin-bottom: .85rem;
  }
  .badge.idle {
    color: var(--muted);
    background: var(--bar);
  }
  .cover-wrap {
    width: 100%%;
    aspect-ratio: 1;
    border-radius: 14px;
    overflow: hidden;
    background: var(--bar);
    margin-bottom: 1rem;
    position: relative;
  }
  .cover-wrap img {
    width: 100%%;
    height: 100%%;
    object-fit: cover;
    display: block;
  }
  .cover-wrap.hidden { display: none; }
  .cover-wrap.placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--muted);
    font-size: 2.5rem;
  }
  .title {
    font-size: clamp(1.15rem, 2.5vw, 1.4rem);
    font-weight: 650;
    line-height: 1.3;
    margin: 0 0 .3rem;
    word-break: break-word;
  }
  .artist {
    color: var(--muted);
    margin: 0 0 1rem;
    font-size: 1rem;
    word-break: break-word;
  }
  .progress {
    display: flex;
    align-items: center;
    gap: .55rem;
    font-variant-numeric: tabular-nums;
    font-size: .8rem;
    color: var(--muted);
    margin-bottom: 1rem;
  }
  .progress.hidden { display: none; }
  .bar {
    flex: 1;
    height: 6px;
    border-radius: 3px;
    background: var(--bar);
    overflow: hidden;
  }
  .bar > i {
    display: block;
    height: 100%%;
    width: 0%%;
    background: var(--bar-fill);
    border-radius: 3px;
    transition: width .4s linear;
  }
  .bio {
    font-size: .9rem;
    line-height: 1.5;
    color: var(--text);
    margin: 0 0 1rem;
    max-height: 9.5rem;
    overflow: auto;
    opacity: .92;
  }
  .bio.hidden { display: none; }
  .meta {
    font-size: .8rem;
    color: var(--muted);
    margin: 0 0 .85rem;
  }
  .meta.hidden { display: none; }
  .links {
    display: flex;
    flex-wrap: wrap;
    gap: .5rem;
    margin-bottom: .85rem;
  }
  .links a {
    color: var(--accent);
    text-decoration: none;
    font-size: .85rem;
    padding: .25rem .55rem;
    border-radius: 8px;
    background: var(--accent-soft);
  }
  .links a:hover { text-decoration: underline; }
  .foot {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
    font-size: .75rem;
    color: var(--muted);
    border-top: 1px solid var(--card-border);
    padding-top: .75rem;
    margin-top: .25rem;
  }
  .foot a { color: var(--muted); text-decoration: none; }
  .foot a:hover { text-decoration: underline; }
  .empty {
    text-align: center;
    padding: 2rem 0.5rem 1.5rem;
    color: var(--muted);
  }
  .empty .icon { font-size: 1.75rem; margin-bottom: .5rem; }
  .hidden { display: none !important; }
</style>
</head>
<body>
  <div class="card" id="app">
    <div class="clock" id="clock">—</div>
    <div id="content">
      <div class="empty" id="empty">
        <div class="icon">💤</div>
        <div>Nothing playing</div>
      </div>
      <div class="media hidden" id="media">
        <div class="cover-wrap hidden" id="coverWrap"><img id="cover" alt="Album cover"/></div>
        <div class="details">
          <div class="badge" id="badge">▶ Playing</div>
          <h1 class="title" id="track"></h1>
          <p class="artist" id="artist"></p>
          <div class="meta hidden" id="agoMeta">Last played · <span id="ago">—</span></div>
          <div class="progress hidden" id="progress">
            <span id="t0">0:00</span>
            <div class="bar"><i id="bar"></i></div>
            <span id="t1">0:00</span>
          </div>
          <p class="bio hidden" id="bio"></p>
          <div class="links" id="links"></div>
        </div>
      </div>
      <div class="foot">
        <span id="emoji"></span>
        <a id="wm" href="#" target="_blank" rel="noopener"></a>
      </div>
    </div>
  </div>
<script>
(function () {
  const PLAYING_URL = %q + "/playing";
  const EVENTS_URL = %q + "/events";
  const $ = (id) => document.getElementById(id);

  let state = { idle: true, playing: false };

  function pad(n) { return String(n).padStart(2, "0"); }

  function formatClock(d) {
    return pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds())
      + " " + pad(d.getDate()) + "." + pad(d.getMonth() + 1) + "." + d.getFullYear();
  }

  function formatAgo(then) {
    if (!(then instanceof Date) || isNaN(then)) return "";
    const now = new Date();
    if (then > now) return "in the future";
    const d = (now - then) / 1000;
    if (d < 5) return "just now";
    if (d < 60) return Math.floor(d) + " sec ago";
    if (d < 3600) return Math.floor(d / 60) + " min ago";
    if (d < 86400) {
      const n = Math.floor(d / 3600);
      return n === 1 ? "1 hour ago" : n + " hours ago";
    }
    if (d < 30 * 86400) {
      const n = Math.floor(d / 86400);
      return n === 1 ? "1 day ago" : n + " days ago";
    }
    if (d < 365 * 86400) {
      let n = Math.floor(d / (30 * 86400));
      if (n < 1) n = 1;
      return n === 1 ? "1 month ago" : n + " months ago";
    }
    let n = Math.floor(d / (365 * 86400));
    if (n < 1) n = 1;
    return n === 1 ? "1 year ago" : n + " years ago";
  }

  function fmtMs(ms) {
    if (ms == null || ms < 0) return "0:00";
    const s = Math.floor(ms / 1000);
    const m = Math.floor(s / 60);
    return m + ":" + pad(s %% 60);
  }

  function trackTime(s) {
    if (s && s.track && s.track.time) return new Date(s.track.time);
    if (s && s.time) return new Date(s.time);
    return null;
  }

  function tickClock() {
    $("clock").textContent = formatClock(new Date());
    const t = trackTime(state);
    if (t) {
      const ago = formatAgo(t);
      const el = $("ago");
      if (el) el.textContent = ago || "—";
    }
  }

  function setCover(url) {
    const wrap = $("coverWrap");
    const img = $("cover");
    if (url) {
      img.src = url;
      wrap.classList.remove("hidden", "placeholder");
      wrap.innerHTML = "";
      wrap.appendChild(img);
    } else {
      img.removeAttribute("src");
      wrap.classList.remove("hidden");
      wrap.classList.add("placeholder");
      wrap.textContent = "🎵";
    }
  }

  function apply(s) {
    state = s || { idle: true, playing: false };
    const idle = !!(state.idle || !state.playing);
    const track = state.track;
    const hasTrack = !!(track && (track.title || track.artist));

    // Empty: no track history at all
    if (!hasTrack) {
      $("empty").classList.remove("hidden");
      $("media").classList.add("hidden");
      document.title = "teletrack — idle";
      $("emoji").textContent = "";
      const wm = $("wm");
      wm.textContent = state.watermark || "";
      wm.href = state.watermark_link || "#";
      tickClock();
      return;
    }

    $("empty").classList.add("hidden");
    $("media").classList.remove("hidden");
    document.body.classList.toggle("layout-wide", window.matchMedia("(min-width: 960px)").matches);

    const badge = $("badge");
    if (idle) {
      badge.textContent = "💤 Nothing playing";
      badge.classList.add("idle");
      document.title = "teletrack — " + (track.artist || "") + " — " + (track.title || "");
    } else {
      badge.textContent = track.playing === false ? "⏸ Paused" : "▶ Playing";
      badge.classList.remove("idle");
      document.title = (track.playing === false ? "⏸ " : "▶ ") + (track.artist || "") + " — " + (track.title || "");
    }

    $("track").textContent = track.title || "";
    $("artist").textContent = track.artist || "";
    setCover(track.cover_url || "");

    const agoMeta = $("agoMeta");
    if (idle) {
      agoMeta.classList.remove("hidden");
      const t = trackTime(state);
      $("ago").textContent = t ? formatAgo(t) : "—";
    } else {
      agoMeta.classList.add("hidden");
    }

    const prog = $("progress");
    if (!idle && track.progress_ms != null && track.duration_ms != null && track.duration_ms > 0) {
      prog.classList.remove("hidden");
      $("t0").textContent = fmtMs(track.progress_ms);
      $("t1").textContent = fmtMs(track.duration_ms);
      const pct = Math.min(100, (100 * track.progress_ms) / track.duration_ms);
      $("bar").style.width = pct.toFixed(2) + "%%";
    } else {
      prog.classList.add("hidden");
    }

    const bio = $("bio");
    const artist = state.artist;
    if (artist && artist.bio) {
      bio.textContent = artist.bio;
      bio.classList.remove("hidden");
    } else {
      bio.textContent = "";
      bio.classList.add("hidden");
    }

    const links = $("links");
    links.innerHTML = "";
    if (track.track_link) {
      const a = document.createElement("a");
      a.href = track.track_link;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      a.textContent = "🎹 " + (track.track_link_service || "Track");
      links.appendChild(a);
    }
    if (artist && artist.link) {
      const a = document.createElement("a");
      a.href = artist.link;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      a.textContent = "🎨 " + (artist.bio_service || "Bio");
      links.appendChild(a);
    }

    $("emoji").textContent = state.emoji || "";
    const wm = $("wm");
    wm.textContent = state.watermark || "";
    wm.href = state.watermark_link || "#";
    tickClock();
  }

  async function pull() {
    try {
      const r = await fetch(PLAYING_URL, { cache: "no-store" });
      if (r.ok) apply(await r.json());
    } catch (_) {}
  }

  function connectSSE() {
    const es = new EventSource(EVENTS_URL);
    es.addEventListener("state", (ev) => {
      try { apply(JSON.parse(ev.data)); } catch (_) {}
    });
    // Fallback: some proxies strip event names
    es.onmessage = (ev) => {
      try { apply(JSON.parse(ev.data)); } catch (_) {}
    };
    es.onerror = () => {
      es.close();
      setTimeout(connectSSE, 2000);
      pull();
    };
  }

  window.matchMedia("(min-width: 960px)").addEventListener("change", () => {
    document.body.classList.toggle("layout-wide", window.matchMedia("(min-width: 960px)").matches);
  });

  pull();
  connectSSE();
  setInterval(pull, 15000);
  setInterval(tickClock, 1000);
  tickClock();
})();
</script>
</body>
</html>
`
