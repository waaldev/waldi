(function () {
  const openBtn = document.querySelector('[data-letter-open]')
  const composer = document.getElementById('letter')

  function openLetter() {
    if (!composer) return
    composer.hidden = false
    const field = composer.querySelector('textarea')
    field?.focus()
    composer.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }

  if (openBtn && composer) {
    openBtn.addEventListener('click', openLetter)
    if (window.location.hash === '#letter') openLetter()
  }

  const shareBtn = document.querySelector('[data-share]')
  if (shareBtn && navigator.share) {
    shareBtn.addEventListener('click', async () => {
      try {
        await navigator.share({ url: window.location.href, title: document.title })
      } catch {
        /* user cancelled */
      }
    })
  } else if (shareBtn) {
    shareBtn.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(window.location.href)
        shareBtn.textContent = shareBtn.dataset.copied || shareBtn.textContent
      } catch {
        /* clipboard unavailable */
      }
    })
  }
})()
