async function activeUrl() {
  const [tab] = await chrome.tabs.query({active: true, currentWindow: true});
  return {tab, url: (tab && tab.url) || ''};
}

function el(html) {
  const d = document.createElement('div');
  d.innerHTML = html;
  return d.firstChild;
}

async function render() {
  const list = document.getElementById('list');
  const status = document.getElementById('status');
  const {tab, url} = await activeUrl();
  document.getElementById('host').textContent = (() => {
    try { return new URL(url).hostname; } catch { return ''; }
  })();
  let logins = [];
  try {
    const res = await chrome.runtime.sendMessage({type: 'get-logins', url});
    if (res && res.ok) logins = res.logins || [];
    else throw new Error((res && res.error) || 'error');
    status.textContent = '';
  } catch (e) {
    const msg = String((e && e.message) || e);
    list.innerHTML = '';
    const hint = msg.includes('not-paired') || msg.includes('not-associated')
      ? 'Pair the extension in CipherSync Settings → Browser extension.'
      : msg.includes('database-locked')
        ? 'Unlock your vault in CipherSync first.'
        : 'Is CipherSync running? (' + msg + ')';
    list.appendChild(el(`<div class="err">${hint}</div>`));
    return;
  }
  list.innerHTML = '';
  if (!logins.length) {
    list.appendChild(el('<div class="empty">No logins for this site.</div>'));
    return;
  }
  for (const l of logins) {
    const row = el(`<button class="row"><span style="min-width:0"><span class="t"></span><br><span class="u"></span></span><span></span></button>`);
    row.querySelector('.t').textContent = l.title || 'Untitled';
    row.querySelector('.u').textContent = l.username || '';
    const right = row.querySelectorAll('span')[2];
    if (l.totp) {
      const b = document.createElement('button');
      b.className = 'totp';
      b.textContent = 'TOTP';
      b.title = 'Copy 2FA code';
      b.onclick = async (e) => {
        e.stopPropagation();
        const r = await chrome.runtime.sendMessage({type: 'get-totp', id: l.id});
        if (r && r.ok && r.totp) await navigator.clipboard.writeText(r.totp);
        window.close();
      };
      right.appendChild(b);
    }
    row.onclick = async () => {
      await chrome.runtime.sendMessage({type: 'fill', tabId: tab.id, username: l.username, password: l.password});
      window.close();
    };
    list.appendChild(row);
  }
}

document.getElementById('lock').onclick = async () => {
  await chrome.runtime.sendMessage({type: 'lock'});
  window.close();
};

render();
