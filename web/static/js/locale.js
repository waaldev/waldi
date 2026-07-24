// Silently corrects the UI language when the visitor's real OS timezone
// disagrees with the server's CF-IPCountry guess. This matters for Iranian
// and Afghan visitors who commonly browse through a VPN to reach the open
// internet - their apparent IP country is the VPN exit node, not Iran/
// Afghanistan, so the server-side guess is often wrong for exactly this
// audience. Never reloads the current page; the correction applies from
// the next page view onward.
(function () {
  var persian = { "Asia/Tehran": 1, "Asia/Kabul": 1 };

  var tz;
  try {
    tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch (e) {
    return;
  }

  var lang = persian[tz] ? "fa" : "en";
  if (lang === document.documentElement.lang) {
    return;
  }

  fetch("/lang/" + lang + "?auto=1", { method: "POST", credentials: "same-origin" });
})();
