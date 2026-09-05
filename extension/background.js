// CipherSync background service worker (MV3).
// Brokers between pages/content scripts and the desktop native host.
// Uses a persistent native port; (re-)associates with the stored assoc id.

const HOST = 'com.ciphersync.host';

let port = null;
let assocId = null;
let pending = new Map();
let reqSeq = 0;
let connecting = null;

async function loadAssoc() {
  const st = await chrome.storage.local.get(['assocId']);
  assocId = st.assocId || null;
}

function connect() {
  if (port) return Promise.resolve(port);
  if (connecting) return connecting;
  connecting = new Promise((resolve, reject) => {
    try {
      const p = chrome.runtime.connectNative(HOST);
      p.onDisconnect.addListener(() => {
        port = null;
        for (const [, rej] of pending) rej(new Error('disconnected'));
        pending.clear();
      });
      p.onMessage.addListener((msg) => {
        const id = msg && msg.requestId;
        if (id && pending.has(id)) {
          const {res} = pending.get(id);
          pending.delete(id);
          res(msg);
        }
      });
      port = p;
      resolve(p);
    } catch (e) {
      reject(e);
    } finally {
      connecting = null;
    }
  });
  return connecting;
}

async function callHost(msg, {needsAssoc = true} = {}) {
  await loadAssoc();
  if (needsAssoc && !assocId) throw new Error('not-paired');
  const p = await connect();
  if (needsAssoc) {
    const t = await rawCall(p, {action: 'test-associate', id: assocId});
    if (!t || !t.success) throw new Error('not-associated');
  }
  return rawCall(p, msg);
}

function rawCall(p, msg) {
  return new Promise((resolve, reject) => {
    const id = 'r' + (++reqSeq) + '-' + Date.now();
    pending.set(id, {res: resolve, rej: reject});
    try {
      p.postMessage({...msg, requestId: id});
    } catch (e) {
      pending.delete(id);
      reject(e);
    }
    setTimeout(() => {
      if (pending.has(id)) {
        pending.delete(id);
        reject(new Error('timeout'));
      }
    }, 15000);
  });
}

async function getLogins(url) {
  const res = await callHost({action: 'get-logins', url});
  if (!res || !res.success) {
    const err = (res && res.error) || 'error';
    throw new Error(err);
  }
  return res.logins || [];
}

async function refreshBadge(tabId, url) {
  try {
    const res = await callHost({action: 'get-logins-count', url});
    const n = (res && res.success) ? res.count : 0;
    chrome.action.setBadgeText({tabId, text: n > 0 ? String(Math.min(n, 99)) : ''});
    chrome.action.setBadgeBackgroundColor({tabId, color: '#4f46e5'});
  } catch {
    chrome.action.setBadgeText({tabId, text: ''});
  }
}

chrome.tabs.onUpdated.addListener((tabId, info, tab) => {
  if (info.status === 'complete' && tab.url && /^https?:/.test(tab.url)) {
    refreshBadge(tabId, tab.url).catch(() => undefined);
  }
});
chrome.tabs.onActivated.addListener(async (info) => {
  try {
    const tab = await chrome.tabs.get(info.tabId);
    if (tab.url && /^https?:/.test(tab.url)) refreshBadge(info.tabId, tab.url).catch(() => undefined);
  } catch {
    // ignore
  }
});

// messages from popup / content scripts
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  (async () => {
    try {
      if (msg.type === 'get-logins') {
        sendResponse({ok: true, logins: await getLogins(msg.url)});
      } else if (msg.type === 'fill') {
        const tabId = sender.tab ? sender.tab.id : msg.tabId;
        await chrome.tabs.sendMessage(tabId, {type: 'fill', username: msg.username, password: msg.password});
        sendResponse({ok: true});
      } else if (msg.type === 'get-totp') {
        const res = await callHost({action: 'get-totp', id: msg.id});
        sendResponse({ok: !!(res && res.success), totp: res && res.totp});
      } else if (msg.type === 'generate-password') {
        const res = await callHost({action: 'generate-password'});
        sendResponse({ok: !!(res && res.success), password: res && res.password});
      } else if (msg.type === 'lock') {
        await callHost({action: 'lock-database'}, {needsAssoc: false}).catch(() => undefined);
        sendResponse({ok: true});
      } else if (msg.type === 'associate') {
        const res = await rawCall(await connect(), {action: 'associate', code: msg.code});
        if (res && res.success && res.id) {
          assocId = res.id;
          await chrome.storage.local.set({assocId: res.id});
          sendResponse({ok: true});
        } else {
          sendResponse({ok: false, error: (res && res.error) || 'error'});
        }
      } else if (msg.type === 'ping') {
        const res = await rawCall(await connect(), {action: 'ping'}).catch(() => null);
        sendResponse({ok: !!(res && res.success)});
      } else {
        sendResponse({ok: false, error: 'unknown'});
      }
    } catch (e) {
      sendResponse({ok: false, error: String((e && e.message) || e)});
    }
  })();
  return true; // async response
});
