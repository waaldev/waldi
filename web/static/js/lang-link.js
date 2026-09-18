(function () {
  function remember(lang, query) {
    try {
      fetch("/lang/" + lang + "?" + query, { method: "POST", credentials: "same-origin", keepalive: true });
    } catch (e) {}
  }

  var lang = document.documentElement.lang;
  if (lang === "fa" && document.cookie.indexOf("waldi_lang_pinned=1") === -1 && document.cookie.indexOf("waldi_lang=fa") === -1) {
    remember("fa", "auto=1");
  }

  document.addEventListener("click", function (e) {
    var link = e.target.closest && e.target.closest("a.lang-toggle");
    if (link) remember(link.getAttribute("hreflang"), "link=1");
  });
})();
