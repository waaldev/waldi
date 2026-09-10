(function () {
  document.querySelectorAll('[data-owner-toggle]').forEach(function (button) {
    button.addEventListener('click', function () {
      var form = document.querySelector('form.' + button.dataset.ownerToggle)
      if (form) form.classList.toggle('open')
    })
  })

  document.querySelectorAll('[data-owner-close]').forEach(function (button) {
    button.addEventListener('click', function () {
      var form = button.closest('form')
      if (form) form.classList.remove('open')
    })
  })
})()
