(function () {
  var root = document.querySelector('[data-public-state]')
  if (!root) return

  var username = root.getAttribute('data-username')
  if (!username) return

  var url = '/api/public-state?username=' + encodeURIComponent(username)
  if (root.hasAttribute('data-state-post')) url += '&post=1'

  function hide(selector) {
    document.querySelectorAll(selector).forEach(function (element) {
      element.hidden = true
    })
  }

  function show(name) {
    var element = root.querySelector('[data-state="' + name + '"]')
    if (element) element.hidden = false
  }

  fetch(url, { credentials: 'same-origin' })
    .then(function (response) { return response.ok ? response.json() : null })
    .then(function (state) {
      if (!state || !state.authenticated) return

      hide('[data-state]')
      hide('[data-anonymous-only]')
      if (state.owner) return

      show(state.following ? 'unfollow' : 'follow')
      if (root.hasAttribute('data-state-post')) {
        show(state.can_send_letter ? 'letter' : 'letter-gated')
        if (state.can_send_letter && new URLSearchParams(window.location.search).get('letter') === 'sent') {
          var composer = document.getElementById('letter')
          if (composer) composer.hidden = false
        }
      }
    })
    .catch(function () {})
})()
