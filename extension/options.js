const msg = document.getElementById('msg');

document.getElementById('pair').onclick = async () => {
  const code = document.getElementById('code').value.trim();
  if (!code) {
    msg.textContent = 'Enter the pairing code first.';
    return;
  }
  msg.textContent = 'Pairing…';
  const res = await chrome.runtime.sendMessage({type: 'associate', code});
  msg.textContent = res && res.ok ? 'Paired! You can close this page.' : 'Failed: ' + ((res && res.error) || 'error');
};

document.getElementById('ping').onclick = async () => {
  msg.textContent = 'Checking…';
  const res = await chrome.runtime.sendMessage({type: 'ping'});
  msg.textContent = res && res.ok ? 'Host reachable.' : 'Host not reachable. Is CipherSync running?';
};
