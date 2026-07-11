(function () {
  var root = document.querySelector('[data-reading-root]')
  if (!root) return

  var impressionID = Number(root.getAttribute('data-impression-id') || '0')
  if (!impressionID) return

  var startedAt = Date.now()
  var maxScroll = 0

  function scrollPercent() {
    var doc = document.documentElement
    var body = document.body
    var scrollTop = window.scrollY || doc.scrollTop || body.scrollTop || 0
    var scrollable = Math.max(1, doc.scrollHeight - window.innerHeight)
    return Math.max(0, Math.min(100, Math.round((scrollTop / scrollable) * 100)))
  }

  function updateScroll() {
    maxScroll = Math.max(maxScroll, scrollPercent())
  }

  function payload() {
    updateScroll()
    var dwell = Math.max(0, Math.round((Date.now() - startedAt) / 1000))
    return {
      impression_id: impressionID,
      max_scroll_pct: maxScroll,
      dwell_seconds: dwell,
      completed: maxScroll >= 90 && dwell >= 10,
    }
  }

  function send() {
    var body = JSON.stringify(payload())
    if (navigator.sendBeacon) {
      navigator.sendBeacon('/api/events/readings', new Blob([body], { type: 'application/json' }))
      return
    }
    fetch('/api/events/readings', {
      method: 'POST',
      body: body,
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      keepalive: true,
    }).catch(function () {})
  }

  window.addEventListener('scroll', updateScroll, { passive: true })
  window.addEventListener('pagehide', send)
  window.setInterval(send, 15000)
  updateScroll()
})()

