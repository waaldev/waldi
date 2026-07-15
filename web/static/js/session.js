(function () {
  const script = document.currentScript
  const appBase = script && script.dataset.appBase
  if (!appBase) return

  // Guards against a bridge/redirect loop if a stale copy of this page
  // (e.g. cached by an upstream proxy) keeps getting re-served after the
  // session was already bridged once in this tab.
  const guardKey = 'waldi_bridge_attempted:' + window.location.href
  try {
    if (window.sessionStorage.getItem(guardKey)) return
    window.sessionStorage.setItem(guardKey, '1')
  } catch (e) {}

  const returnTo = window.location.href
  const opts = { credentials: 'include' }

  fetch(appBase + '/api/me', opts)
    .then((response) => (response.ok ? response.json() : null))
    .then((data) => {
      if (!data || !data.username) return
      const url =
        appBase +
        '/api/auth/bridge?return=' +
        encodeURIComponent(returnTo)
      return fetch(url, opts).then((response) =>
        response.ok ? response.json() : null
      )
    })
    .then((data) => {
      if (data && data.continue) {
        window.location.replace(data.continue)
      }
    })
    .catch(() => {})
})()
