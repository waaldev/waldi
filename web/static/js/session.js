(function () {
  const script = document.currentScript
  const appBase = script && script.dataset.appBase
  if (!appBase) return

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
