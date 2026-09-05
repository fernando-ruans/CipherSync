// CipherSync content script: finds login forms and fills credentials.
// Runs in all frames; only acts when asked by the background/popup.

function isVisible(el) {
  if (!el || !(el instanceof HTMLElement)) return false;
  const r = el.getBoundingClientRect();
  if (r.width === 0 || r.height === 0) return false;
  const s = getComputedStyle(el);
  return s.display !== 'none' && s.visibility !== 'hidden' && !el.disabled && el.type !== 'hidden';
}

function setNativeValue(el, value) {
  const proto = el instanceof HTMLTextAreaElement
    ? HTMLTextAreaElement.prototype
    : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, 'value').set;
  setter.call(el, value);
  el.dispatchEvent(new Event('input', {bubbles: true}));
  el.dispatchEvent(new Event('change', {bubbles: true}));
  el.dispatchEvent(new KeyboardEvent('keydown', {bubbles: true}));
}

function findPasswordField() {
  const cands = Array.from(document.querySelectorAll('input[type="password"]')).filter(isVisible);
  return cands[0] || null;
}

function findUsernameField(pw) {
  if (!pw) return null;
  const form = pw.form;
  const pool = form
    ? Array.from(form.querySelectorAll('input')).filter(isVisible)
    : Array.from(document.querySelectorAll('input')).filter(isVisible);
  // prefer explicit autocomplete, then email/text near the password field
  const byAuto = pool.find((el) =>
    ['username', 'email'].includes((el.getAttribute('autocomplete') || '').toLowerCase()) &&
    el.type !== 'password' && el.type !== 'hidden'
  );
  if (byAuto) return byAuto;
  const idx = pool.indexOf(pw);
  for (let i = idx - 1; i >= 0; i--) {
    const el = pool[i];
    if (['text', 'email'].includes(el.type)) return el;
  }
  return pool.find((el) => ['text', 'email'].includes(el.type)) || null;
}

// observe SPA-injected forms so a later fill still finds fields
let observed = false;
function ensureObserver() {
  if (observed) return;
  observed = true;
  try {
    new MutationObserver(() => undefined).observe(document.documentElement, {
      childList: true,
      subtree: true,
    });
  } catch {
    // ignore
  }
}
ensureObserver();

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg && msg.type === 'fill') {
    try {
      const pw = findPasswordField();
      if (!pw) {
        sendResponse({ok: false, error: 'no-field'});
        return;
      }
      const user = findUsernameField(pw);
      if (user && msg.username !== undefined) setNativeValue(user, msg.username);
      setNativeValue(pw, msg.password || '');
      pw.focus();
      sendResponse({ok: true});
    } catch (e) {
      sendResponse({ok: false, error: String(e)});
    }
    return true;
  }
});
