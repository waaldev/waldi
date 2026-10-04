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
    if (!link) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) {
      remember(link.getAttribute("hreflang"), "link=1");
      return;
    }
    e.preventDefault();
    var went = false;
    function go() {
      if (went) return;
      went = true;
      location.href = link.href;
    }
    setTimeout(go, 1500);
    try {
      fetch("/lang/" + link.getAttribute("hreflang") + "?link=1", { method: "POST", credentials: "same-origin" }).then(go, go);
    } catch (err) {
      go();
    }
  });
})();
