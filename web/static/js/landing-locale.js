(function () {
  if (document.cookie.indexOf("waldi_lang_pinned=1") !== -1) {
    return;
  }
  var tz;
  try {
    tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch (e) {
    return;
  }
  if (tz === "Asia/Tehran") {
    location.replace("/fa" + location.search + location.hash);
  }
})();
