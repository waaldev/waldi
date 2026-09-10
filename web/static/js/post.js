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
  async function copyLink() {
    let copied = false
    if (navigator.clipboard) {
      try {
        await navigator.clipboard.writeText(window.location.href)
        copied = true
      } catch (e) {}
    }
    if (!copied) {
      const input = document.createElement('textarea')
      input.value = window.location.href
      input.setAttribute('readonly', '')
      input.style.position = 'fixed'
      input.style.opacity = '0'
      document.body.appendChild(input)
      input.select()
      copied = document.execCommand('copy')
      input.remove()
    }
    if (copied) shareBtn.textContent = shareBtn.dataset.copied || shareBtn.textContent
  }

  if (shareBtn) {
    shareBtn.addEventListener('click', async () => {
      if (navigator.share) {
        try {
          await navigator.share({ url: window.location.href, title: document.title })
          return
        } catch (error) {
          if (error && error.name === 'AbortError') return
        }
      }
      await copyLink()
    })
  }
})()
